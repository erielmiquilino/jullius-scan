## Why

Hoje o app permite abrir os detalhes de um Scan já persistido, mas não oferece nenhuma forma de remover um lançamento incorreto ou duplicado. Isso força manutenção manual no banco e deixa a lista principal desatualizada em relação ao que o usuário espera conseguir gerenciar pelo próprio app.

## What Changes

- Adicionar exclusão autenticada de recibos/Scans pelo app, iniciada a partir da tela de detalhes do registro.
- Expor um endpoint protegido `DELETE /api/v1/receipts/{id}` com validação por `House` e remoção segura dos dados relacionados.
- Atualizar o fluxo mobile para exibir uma confirmação explícita antes da exclusão e bloquear ações duplicadas enquanto a remoção estiver em andamento.
- Após exclusão bem-sucedida, retornar para a listagem principal e recarregar os registros sem o item removido.
- Cobrir o fluxo com testes E2E/backend e testes do mobile para garantir confirmação, deleção e atualização da lista.

## Capabilities

### New Capabilities
- `mobile-receipt-deletion`: permite excluir um recibo a partir da tela de detalhes com confirmação explícita, retorno à listagem e refresh dos dados.

### Modified Capabilities
- `receipt-ingestion-api`: adiciona deleção autenticada de recibos por `House`, incluindo tratamento consistente para vínculos com `scraping_jobs`.

## Impact

- **Backend (`backend/`)**:
  - `internal/api/handlers.go` e `router.go` para expor `DELETE /api/v1/receipts/{id}`.
  - `internal/database/queries.go` para remover recibos de forma transacional e limpar referências em `scraping_jobs.receipt_id`.
  - `migrations/003_receipt_deletion_fk.*.sql` para ajustar a foreign key de `scraping_jobs.receipt_id` e permitir exclusão sem violar integridade.
- **Mobile (`mobile/`)**:
  - `lib/services/api_client.dart` para adicionar `deleteReceipt(id)`.
  - `lib/screens/receipt_detail_screen.dart` para adicionar CTA de remoção, diálogo de confirmação e estado de loading.
  - `lib/screens/home_screen.dart` para reagir ao retorno da tela de detalhes e recarregar a listagem.
- **Testes**:
  - `testing/e2e/backend/` para validar o endpoint de deleção e os efeitos de integridade.
  - `mobile/test/` para validar confirmação, navegação de volta e atualização da lista.
