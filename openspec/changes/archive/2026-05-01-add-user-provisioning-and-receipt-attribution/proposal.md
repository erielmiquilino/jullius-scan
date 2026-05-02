## Why

Hoje o provisionamento de um novo usuário da House é manual em duas etapas desconectadas: criar o usuário no Firebase Console pela UI e depois rodar `seed_mvp.sql` ajustando o UID na mão. Como o app é de uso pessoal e não terá tela de cadastro, esse fluxo precisa caber em um único comando.

Além disso, hoje os cupons listados/exibidos no app não mostram quem fez o lançamento. Em uma House compartilhada (ex.: dois moradores), saber quem registrou cada cupom é informação útil para revisão e responsabilização das compras, e o backend já rastreia `submitted_by` em `scraping_jobs` — falta apenas expor isso no recibo persistido e na visualização.

## What Changes

- Adicionar um comando Go em `backend/cmd/provision-user` que recebe email e senha (e opcionalmente nome) e:
  - Cria o usuário no Firebase Auth via Admin SDK (`auth.CreateUser`).
  - Insere o registro em `users` (Firebase UID, email, nome) e em `house_members` vinculando à House padrão (resolvida por nome ou por id via flag).
  - É idempotente: se o usuário já existir no Firebase ou no banco, reaproveita os registros e garante a `house_members`.
- Aposentar o passo manual de editar `seed_mvp.sql` para cada novo usuário (o seed continua valendo só para o owner inicial / bootstrap da House).
- Persistir `created_by` (FK para `users.id`) em `receipts` via nova migration, populando-o no worker no momento do `CreateReceipt` a partir do `scraping_jobs.submitted_by` do job que originou o cupom. Receitas existentes recebem backfill a partir do job mais antigo associado àquele `receipt_id`.
- Expor o submitter apenas em `GET /api/v1/receipts/{id}` como um sub-objeto `submitted_by` com `id`, `name`, `email` — opcional/`omitempty` quando ausente (cupons antigos sem backfill possível). A listagem `GET /api/v1/receipts` permanece inalterada.
- Renderizar uma linha discreta "Lançado por <Nome>" no cabeçalho da tela de detalhe do cupom (logo abaixo da data/total), no app Flutter.
- Aceitar a senha do novo usuário via `--password <pwd>` ou `--password-stdin` no CLI; em caso de falha do INSERT no Postgres após criação bem-sucedida no Firebase, o CLI **não** apaga o usuário no Firebase: imprime o UID e orienta uma re-execução (que é idempotente).

## Capabilities

### New Capabilities
- `user-provisioning-cli`: Comando administrativo executável localmente que cria atomicamente um usuário no Firebase Auth e na base de dados da aplicação, vinculando-o a uma House existente.

### Modified Capabilities
- `receipt-ingestion-api`: Receitas passam a carregar a identidade do membro da House que submeteu o cupom; respostas de receipt incluem o submitter; o worker persiste essa autoria a partir do job que originou o cupom.

## Impact

- Código:
  - `backend/cmd/provision-user/main.go` (novo binário CLI).
  - `backend/internal/database/queries.go` — novas queries para upsert de user e membership; leitura/escrita de `created_by`.
  - `backend/migrations/008_receipt_created_by.up.sql` / `.down.sql` — nova coluna `receipts.created_by` + backfill.
  - `backend/internal/scraper/worker.go` (ou onde `CreateReceipt` é chamado) — propagar `submitted_by` do job para o receipt.
  - `backend/internal/api/handlers.go` — `ReceiptResponse` ganha campo `submitted_by` opcional.
  - `mobile/lib/features/receipts/.../receipt_detail_screen.dart` — exibe submitter; modelo de receipt ganha campo nullable.
- Dependências:
  - Reusa `firebase.google.com/go/v4` já presente em `backend/go.mod`. Requer um service-account JSON do Firebase via `GOOGLE_APPLICATION_CREDENTIALS` para o CLI.
- Operação:
  - `backend/scripts/provision_mvp.sh` continua válido para o bootstrap inicial; o novo CLI substitui apenas o passo de adicionar membros adicionais.
- Testes:
  - E2E: novo cenário que injeta um receipt produzido por um job com `submitted_by` e valida que a resposta da API contém o bloco `submitted_by`.
- Sem breaking changes na API (campo é adicionado e marcado `omitempty`).
