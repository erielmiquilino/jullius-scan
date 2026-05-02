## Context

O Jullius Scan é um app de uso pessoal sem fluxo de auto-cadastro: a House e seus membros são provisionados manualmente. Hoje o caminho exige criar o usuário no Firebase Console, copiar o UID e editar `seed_mvp.sql` à mão antes de rodar `provision_mvp.sh`. Para uma House compartilhada por dois ou três moradores, esse processo é frágil e desalinhado entre Firebase e Postgres.

Adicionalmente, o backend já registra `scraping_jobs.submitted_by` (FK para `users.id`), mas essa informação nunca aparece nas respostas de `GET /api/v1/receipts/{id}` nem em `ListReceipts`. O app mobile não tem como mostrar quem registrou cada cupom. Uma vez que o `receipts.fiscal_url` é único por House e o FK `scraping_jobs.receipt_id` é `ON DELETE SET NULL` (i.e., jobs sobrevivem ao cupom mas o caminho inverso é fraco), reusar a relação via job no momento de leitura é tecnicamente possível, mas frágil quando há reenvios e quando a deleção/recriação do cupom faz o link via job ficar ambíguo.

Constraints relevantes:
- Stack Go 1.23 + chi + pgx; SDK Firebase Admin já está em `go.mod` (`firebase.google.com/go/v4`).
- Migrations rodam automaticamente no startup do API (`internal/database/migrate.go`); nunca editar uma migration commitada.
- Toda SQL fica em `internal/database/queries.go`.
- Convenção de erros e logging: `slog` + `fmt.Errorf("action: %w", err)`.
- Mobile usa `StatefulWidget` puro e modelos imutáveis com `factory fromJson` em `snake_case`.

## Goals / Non-Goals

**Goals:**
- Um único comando administrativo (executável Go) provisiona um novo membro: cria no Firebase Auth, insere em `users`, vincula em `house_members` à House escolhida.
- Idempotência: rodar duas vezes com o mesmo email não duplica nem falha — reaproveita Firebase UID e linhas existentes.
- Persistir `created_by` em `receipts` para tornar a autoria do cupom uma propriedade estável do próprio cupom, independente do ciclo de vida dos jobs.
- Backfill determinístico para receitas existentes a partir do job mais antigo associado.
- Resposta JSON da API expõe `submitted_by` (id, name, email) sem quebrar clientes atuais (campo opcional / `omitempty`).
- Tela de detalhe do cupom exibe "Lançado por <Nome>" quando disponível.

**Non-Goals:**
- Não há UI de cadastro/recuperação de senha no app: tudo é offline pelo CLI.
- Não há gestão de papéis (apenas `member` por padrão; `owner` continua sendo definido pelo seed inicial).
- Não há reset de senha, edição de email, ou desligamento de membro via CLI nesta change (escopo: apenas adicionar).
- Não há mudança no contrato de submissão de cupom — quem lança continua sendo identificado pelo Firebase Auth Bearer token; a única mudança é a propagação para `receipts.created_by` no momento da criação.
- Não exibimos `submitted_by` em listagens de busca de itens, busca histórica, ou em outras visões além da listagem e detalhe de cupom.

## Decisions

### 1) Provisionamento como binário Go em `backend/cmd/provision-user`

Usar um comando Go é preferível a um script bash + Node/Python porque:
- O Firebase Admin SDK já está no projeto (`firebase.google.com/go/v4`); não introduz dependência extra.
- Reaproveita `internal/database` e `internal/config`, garantindo que conexão, schema e validações sigam exatamente o que a API usa.
- A execução é cross-platform (Windows é o ambiente principal de dev) sem depender de `psql`/bash disponível.

**Alternativas consideradas:**
- Bash + `firebase` CLI: requer Node + login interativo; quebra em CI e em Windows nativo.
- Endpoint admin protegido na própria API: aumenta superfície de ataque para um app pessoal e exige autenticação especial; o CLI roda apenas localmente com credenciais de service account.

**Forma de invocação:**
```
go run ./cmd/provision-user --email <addr> --password <pwd> [--name "<nome>"] [--house-id <id> | --house-name "<nome>"] [--role member]
```
Variáveis necessárias: `DATABASE_URL`, `GOOGLE_APPLICATION_CREDENTIALS` (path do JSON da service account com permissão `Firebase Authentication Admin`), `FIREBASE_PROJECT_ID`.

### 2) Idempotência por email + Firebase UID

Antes de criar no Firebase, o CLI tenta `auth.GetUserByEmail`. Se o usuário existir, reaproveita o UID retornado em vez de tentar criar novamente. Para a base, usa `INSERT ... ON CONFLICT (firebase_id) DO UPDATE SET email = EXCLUDED.email, name = EXCLUDED.name RETURNING id` — atualiza email/nome se mudaram e retorna o `users.id` em ambos os caminhos. A ligação em `house_members` usa `ON CONFLICT (user_id, house_id) DO NOTHING`. Resultado: rodar o CLI N vezes converge para o mesmo estado.

A senha enviada **só** vale para a criação inicial: se o user já existir no Firebase, a senha é ignorada e um aviso é logado. Para alterar senha existente, esta change não cobre — o usuário pode fazer pelo Console se necessário.

### 3) `created_by` denormalizado em `receipts` (em vez de derivar do job)

**Opção escolhida:** adicionar coluna `created_by BIGINT REFERENCES users(id) ON DELETE SET NULL` em `receipts`.

**Por quê:**
- O FK `scraping_jobs.receipt_id` é `ON DELETE SET NULL` (migration 003): se um cupom for deletado e re-submetido, o job antigo perde o vínculo, e o job novo passa a apontar para um novo receipt — o "lançador" do receipt atual fica claro.
- Ao denormalizar, leitura simples: o handler `GetReceipt` já carrega o receipt e basta um JOIN em `users`. Evita query auxiliar para descobrir o job e tratar "qual job?" quando múltiplos existem.
- O campo é semanticamente parte do receipt (a foto fiscal), não do job (mecanismo de scraping). Quando um futuro `archive_jobs` ou TTL nos jobs entrar, a autoria não desaparece com o histórico.

**Alternativa rejeitada:** JOIN com `scraping_jobs` no momento da leitura. Funciona, mas exige LEFT JOIN com filtragem (ex.: `ORDER BY created_at ASC LIMIT 1`) e amplia a superfície de mudanças se a tabela de jobs evoluir. Custo de migrar é o mesmo.

### 4) Population path: worker preenche `created_by` ao criar o receipt

O ponto natural é a função que persiste o cupom após o scraping bem-sucedido (`ReceiptQueries.CreateReceipt`, hoje sem o campo). Ela passa a aceitar um `submittedBy int64` adicional (ou `*int64`) e usa o `scraping_jobs.submitted_by` do job em execução. O worker já tem o job em mãos, então o repasse é direto e local — sem mudanças em scraping/parsing.

**Backfill (uma única vez na migration up):**
```sql
UPDATE receipts r
SET created_by = sub.submitted_by
FROM (
    SELECT receipt_id, submitted_by
    FROM scraping_jobs
    WHERE receipt_id IS NOT NULL
      AND submitted_by IS NOT NULL
    ORDER BY receipt_id, created_at ASC
) sub
WHERE r.id = sub.receipt_id
  AND r.created_by IS NULL;
```
A subquery escolhe o job mais antigo (primeiro a registrar). Usar `DISTINCT ON (receipt_id)` na subquery garante exatamente uma linha por `receipt_id`.

### 5) API: bloco `submitted_by` opcional em `ReceiptResponse` apenas no detalhe

Estrutura nova:
```go
type SubmittedByResponse struct {
    ID    int64  `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type ReceiptResponse struct {
    // ...campos existentes...
    SubmittedBy *SubmittedByResponse `json:"submitted_by,omitempty"`
}
```
A query usada por `GetByID` em `ReceiptQueries` é estendida com `LEFT JOIN users u ON u.id = r.created_by` retornando colunas adicionais nullable. **`ListByHouse` permanece inalterado** — a listagem não traz `submitted_by` (decisão tomada no momento do desenho: o requisito atual é mostrar o autor apenas na tela de detalhe; se no futuro a lista precisar do campo, basta um JOIN equivalente). O `ReceiptResponse` mantém retro-compatibilidade pelo `omitempty` quando `created_by IS NULL` (cupons que não puderam ser backfilled, p.ex. jobs deletados antes do backfill).

### 6) Mobile: campo opcional no modelo + linha discreta no cabeçalho do detalhe

`Receipt` (modelo) ganha campo opcional `submittedBy` (classe imutável `SubmittedBy { int id; String name; String email; }`) com `factory SubmittedBy.fromJson`. A tela de detalhe renderiza, **logo abaixo da data e do total no cabeçalho do cupom**, uma linha discreta tipo `"Lançado por ${receipt.submittedBy!.name}"` apenas quando o campo está presente; se ausente, a linha é omitida (evita placeholder ruidoso para receipts antigos sem autor). A listagem segue inalterada — não exibe nem consome `submitted_by`.

### 7) Senha em linha de comando — nota de segurança

O CLI aceita as **duas** formas, mutuamente exclusivas:
- `--password <pwd>`: prática para uso pessoal local; risco conhecido de aparecer em `ps` e no histórico do shell.
- `--password-stdin`: a senha é lida via stdin (`echo 'pwd' | provision-user --password-stdin ...`), recomendada quando a sessão pode ser inspecionada.

Quando ambas forem informadas, o CLI rejeita com erro de uso. Quando nenhuma for informada e o usuário a ser criado **ainda não existe** no Firebase, o CLI também falha pedindo uma das duas opções. Quando o usuário **já existe** no Firebase, a senha é opcional e ignorada com warning (a CLI não troca senhas).

### 8) Falha parcial: Firebase OK, Postgres falha

Se `auth.CreateUser` retornar sucesso e o `INSERT` em `users` falhar (ex.: indisponibilidade transitória do Postgres, conflito raro):
- O CLI **não** chama `auth.DeleteUser` para reverter.
- Loga em nível `error` o UID criado e a operação que falhou, e sai com código não-zero.
- Imprime mensagem instruindo o operador a re-executar com os mesmos argumentos: o caminho idempotente (decisão #2) detecta o UID existente, pula a criação no Firebase e continua a partir do `INSERT`.

Justificativa: rollback automático no Firebase pode também falhar (rede instável é justamente o cenário em que o INSERT falhou) e produzir um estado mais difícil de diagnosticar. Reexecução idempotente é uma cura segura.

## Risks / Trade-offs

- **Risco:** Service account JSON do Firebase exposto acidentalmente em repositório.
  - **Mitigação:** Documentar uso de `GOOGLE_APPLICATION_CREDENTIALS` apontando para arquivo fora do repo; adicionar `*.serviceaccount.json` em `.gitignore` se ainda não estiver.

- **Risco:** Backfill atribui o cupom ao primeiro submitter mesmo quando o cupom atual foi recriado por outro membro da House (após exclusão/re-submissão).
  - **Mitigação:** Como a ordem dos jobs por `receipt_id` (após migration `003`) reflete apenas jobs que terminaram apontando para esse receipt, e novas execuções pós-deleção criam novo `receipt_id`, o cenário é raro. Se acontecer, tratamos como dado histórico aceitável; o campo é apenas informativo.

- **Risco:** `--password` em flag fica visível em `ps`/histórico.
  - **Mitigação:** Suporte a `--password-stdin` como opção segura recomendada na documentação.

- **Risco:** CLI rodado contra a House errada provisiona um membro com acesso indevido a receitas alheias.
  - **Mitigação:** Exigir explicitamente `--house-id` ou `--house-name`; sem default. Logar com `slog` o nome resolvido antes de inserir, com confirmação `--yes` (não-interativo) ou prompt interativo.

- **Trade-off:** Denormalizar `created_by` exige uma migration e uma alteração na assinatura de `CreateReceipt` no worker. Custo aceito porque simplifica leituras e desacopla a autoria do ciclo de vida do job.

- **Trade-off:** Não expor o submitter na listagem (`ListByHouse`) significa que, se a UI mobile decidir exibir o autor no resumo do cupom, será preciso uma segunda mudança no backend. Aceito porque mantém o escopo focado no requisito atual (mostrar só no detalhe) e evita o JOIN em listagens.

## Migration Plan

1. Mergear migration `008_receipt_created_by.up.sql` que adiciona a coluna nullable e roda o backfill no mesmo arquivo.
2. Deploy do backend com `created_by` populado para novos receipts e exposto na API.
3. Mergear app mobile com leitura tolerante a `submitted_by` ausente.
4. Anunciar internamente o uso do CLI; manter `seed_mvp.sql` apenas para bootstrap inicial.

**Rollback:** Em caso de regressão, executar `008_receipt_created_by.down.sql` (DROP COLUMN). Backend continua funcional sem o campo (rota retorna `omitempty`). Mobile precisa ignorar o campo, o que já acontece pelo nullable.

## Open Questions

Nenhuma — todas as escolhas relevantes foram fechadas:
- Escopo da API: apenas o detalhe expõe `submitted_by`.
- Falha parcial: o CLI mantém o usuário criado no Firebase e instrui re-execução idempotente.
- UX no detalhe do cupom: linha discreta logo abaixo da data/total no cabeçalho.
- Senha: aceita `--password` e `--password-stdin` (mutuamente exclusivas).
- Tratamento de erro do service account: o CLI propaga e formata o erro retornado pelo Firebase Admin SDK; não há lista pré-validada de permissões — o erro do SDK já é específico.
