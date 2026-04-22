## Context

O backend hoje expõe apenas `POST /api/v1/receipts`, `GET /api/v1/receipts` e `GET /api/v1/receipts/{id}`. No mobile, `HomeScreen` lista os registros e `ReceiptDetailScreen` carrega o detalhe, mas não existe ação de remoção nem retorno com refresh quando o usuário sai dessa tela.

Do lado relacional, `items` já usa `ON DELETE CASCADE` com `receipts`, mas `scraping_jobs.receipt_id` referencia `receipts(id)` sem ação explícita de deleção. Isso significa que apagar um recibo hoje pode falhar por integridade referencial, além de deixar ambíguo o comportamento histórico dos jobs concluídos que originaram aquele registro.

Constraints relevantes:
- A autorização continua house-scoped: um usuário só pode remover recibos da própria `House`.
- O app usa `StatefulWidget` com `setState`, então o fluxo deve respeitar esse padrão sem introduzir gerenciamento de estado novo.
- A lista principal já possui `_loadReceipts()`, então o caminho mais simples é sinalizar deleção bem-sucedida no `Navigator.pop` e disparar um novo fetch.

## Goals / Non-Goals

**Goals:**
- Permitir exclusão de um recibo a partir da tela de detalhes.
- Exigir confirmação explícita antes da remoção.
- Garantir que a API remova o recibo sem violar a integridade com `items` e `scraping_jobs`.
- Fazer o app voltar para a listagem e exibir a lista atualizada sem o item excluído.
- Cobrir o comportamento com testes automatizados de backend e mobile.

**Non-Goals:**
- Adicionar lixeira, soft delete ou recuperação de recibos removidos.
- Excluir `stores` órfãs nesta mesma change.
- Alterar o fluxo de submissão/idempotência além do necessário para manter consistência após a deleção.
- Introduzir paginação, cache local ou atualização otimista na lista.

## Decisions

### Decisão 1: Excluir o recibo com endpoint REST dedicado
Adicionar `DELETE /api/v1/receipts/{id}` como operação autenticada e house-scoped.

**Por quê:** a remoção é uma mudança de estado no recurso `receipt` já exposto pela API. Um endpoint dedicado mantém o contrato previsível para mobile e testes, reaproveitando o mesmo padrão de autorização usado no `GET`.

**Alternativa considerada:** remover via endpoint de job ou via ação implícita em `POST`. Rejeitada porque mistura conceitos diferentes: o recurso removido é o recibo persistido, não o job.

### Decisão 2: Preservar o histórico do job e soltar a referência ao recibo
Ao excluir um recibo, a API deve limpar `scraping_jobs.receipt_id` para os jobs vinculados e só então remover o registro de `receipts` dentro de uma transação.

**Por quê:** o job continua sendo um evento histórico útil para diagnóstico, mas o vínculo para o recibo excluído não pode permanecer apontando para um registro inexistente. A solução mais simples e coerente é permitir `NULL` nessa relação histórica.

**Alternativa considerada:** excluir também os jobs associados. Rejeitada porque apaga histórico operacional e complica mais o comportamento esperado do usuário do que o problema pede.

### Decisão 3: Ajustar a FK para `ON DELETE SET NULL`
Criar uma nova migração para substituir a constraint de `scraping_jobs.receipt_id` por uma versão com `ON DELETE SET NULL`.

**Por quê:** isso protege a integridade mesmo se outra rotina apagar recibos fora do fluxo principal da API e evita depender apenas de uma ordem manual de updates/deletes no código.

**Alternativa considerada:** manter a FK atual e sempre dar `UPDATE scraping_jobs SET receipt_id = NULL` antes do `DELETE`. Parcialmente viável, mas mais frágil a mudanças futuras e menos alinhado ao comportamento desejado do banco.

### Decisão 4: Confirmação por `AlertDialog` modal antes da deleção
Na `ReceiptDetailScreen`, o botão de remoção deve abrir um diálogo de confirmação com pergunta explícita ao usuário antes de chamar a API.

**Por quê:** o requisito pede confirmação clara e o `AlertDialog` é o padrão mais simples e consistente com Material no app atual.

**Alternativa considerada:** `SnackBar` com undo ou `BottomSheet`. Rejeitada porque confirmações destrutivas pedem intenção explícita antes da ação, não compensação posterior.

### Decisão 5: Retorno da detail screen via resultado booleano
Após exclusão bem-sucedida, `ReceiptDetailScreen` deve executar `Navigator.pop(context, true)`; `HomeScreen` passa a aguardar esse resultado e chama `_loadReceipts()` quando receber `true`.

**Por quê:** o app já usa esse padrão em outros fluxos (`scan`/`submit`), então a menor mudança correta é reaproveitar a mesma sinalização de sucesso para refresh.

**Alternativa considerada:** remover o item localmente da lista sem novo fetch. Rejeitada porque exige sincronização extra e pode divergir do estado real da API.

## Risks / Trade-offs

- **[Risco]** Jobs concluídos deixam de referenciar um `receipt_id` após a exclusão. **Mitigação:** manter `status`, `fiscal_url` e demais metadados do job; documentar no spec que a exclusão remove apenas o recibo persistido.
- **[Risco]** Exclusão acidental por toque indevido. **Mitigação:** exigir confirmação modal e mostrar estado de loading para evitar toque repetido.
- **[Risco]** O usuário voltar para a lista antes do refresh terminar e ver estado transitório. **Mitigação:** disparar `_loadReceipts()` imediatamente ao receber o resultado da detail screen e manter o indicador de carregamento já existente.
- **[Trade-off]** A lista será recarregada da API após a exclusão, não atualizada otimisticamente. Isso adiciona uma requisição extra, mas reduz complexidade e inconsistências.

## Migration Plan

1. Criar `003_receipt_deletion_fk.up.sql` removendo a constraint atual de `scraping_jobs.receipt_id` e recriando-a com `ON DELETE SET NULL`.
2. Expor o novo `DELETE /api/v1/receipts/{id}` na API e implementar a deleção transacional em `ReceiptQueries`.
3. Publicar a atualização do mobile com o botão de remoção e a confirmação modal.
4. Rollback: reverter o app para a versão anterior e aplicar `003_receipt_deletion_fk.down.sql`; antes do rollback da FK, garantir que não existam referências inválidas em `scraping_jobs.receipt_id`.

## Open Questions

- O texto do diálogo deve chamar o registro de "Scan", "recibo" ou ambos? Proposta atual: UI orientada a usuário com "remover este lançamento".
- Após a exclusão, vale exibir um `SnackBar` curto na listagem confirmando sucesso? Não é necessário para a capability, mas pode melhorar feedback.
