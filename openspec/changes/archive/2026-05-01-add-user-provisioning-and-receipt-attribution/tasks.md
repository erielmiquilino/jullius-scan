## 1. Database migration: receipts.created_by

- [x] 1.1 Criar `backend/migrations/008_receipt_created_by.up.sql` adicionando coluna `created_by BIGINT REFERENCES users(id) ON DELETE SET NULL` em `receipts`, com índice `idx_receipts_created_by`.
- [x] 1.2 No mesmo `008_*.up.sql`, executar backfill com `UPDATE receipts ... FROM (SELECT DISTINCT ON (receipt_id) receipt_id, submitted_by FROM scraping_jobs WHERE receipt_id IS NOT NULL AND submitted_by IS NOT NULL ORDER BY receipt_id, created_at ASC)` para popular `created_by` a partir do job mais antigo associado.
- [x] 1.3 Criar `backend/migrations/008_receipt_created_by.down.sql` com `DROP INDEX` e `ALTER TABLE receipts DROP COLUMN created_by`.
- [x] 1.4 Atualizar `backend/internal/domain/models.go`: campo `CreatedBy *int64` no struct `Receipt`.
- [x] 1.5 Subir o stack local (`docker compose -f backend/tools/docker-compose.yml up -d`) e validar manualmente que a migration aplica e o backfill popula linhas existentes. (Stack local rebuildado; migration aplicou; smoke confirmou criação de cupom com `created_by` populado.)

## 2. Worker propaga submitter ao criar receipt

- [x] 2.1 Atualizar `ReceiptQueries.CreateReceipt` em `backend/internal/database/queries.go` para incluir `created_by` no INSERT a partir do campo do struct `Receipt`.
- [x] 2.2 No worker (`backend/internal/scraper/worker.go` ou onde `CreateReceipt` é chamado após scraping bem-sucedido), preencher `receipt.CreatedBy = &job.SubmittedBy` antes da persistência.
- [x] 2.3 Rodar `go vet ./...` e `gofmt -l .` no `backend/`.

## 3. API expõe submitted_by no detalhe do recibo

- [x] 3.1 Adicionar tipo `SubmittedByResponse{ID, Name, Email}` em `backend/internal/api/handlers.go`.
- [x] 3.2 Adicionar campo `SubmittedBy *SubmittedByResponse \`json:"submitted_by,omitempty"\`` em `ReceiptResponse`.
- [x] 3.3 Em `ReceiptQueries.GetByID` (ou função auxiliar nova `GetByIDWithSubmitter`) trocar a query por um `LEFT JOIN users u ON u.id = r.created_by` retornando `u.id`, `u.name`, `u.email` como nullable.
- [x] 3.4 Atualizar `toReceiptResponse` (ou criar overload) para popular `SubmittedBy` quando os campos retornados não forem nulos.
- [x] 3.5 Atualizar `Handlers.GetReceipt` em `handlers.go` para usar a query estendida e passar o submitter para `toReceiptResponse`.
- [x] 3.6 Garantir que `Handlers.ListReceipts` permanece inalterado e que `ReceiptResponse` retornado pela listagem **não** inclui `submitted_by` (campo `omitempty` resolve quando `nil`).

## 4. CLI de provisionamento de usuário

- [x] 4.1 Criar diretório `backend/cmd/provision-user/` com `main.go`.
- [x] 4.2 Definir flags: `--email`, `--password`, `--password-stdin`, `--name`, `--house-id`, `--house-name`, `--role` (default `member`), `--yes` (skip confirmação interativa).
- [x] 4.3 Validações iniciais: email obrigatório; House identificada por `--house-id` OU `--house-name` (XOR); senha obrigatória apenas quando o usuário ainda não existe no Firebase; `--password` e `--password-stdin` mutuamente exclusivas.
- [x] 4.4 Carregar config via `internal/config` (reutiliza `DATABASE_URL` e `FIREBASE_PROJECT_ID`); inicializar Firebase Admin SDK via `firebase.NewApp` lendo `GOOGLE_APPLICATION_CREDENTIALS`.
- [x] 4.5 Resolver House: query `SELECT id, name FROM houses WHERE id = $1` ou `WHERE name = $1`; se nome retornar 0 ou >1 linhas, abortar com erro claro.
- [x] 4.6 Lookup no Firebase: `auth.GetUserByEmail`; se existir, reusar UID e logar warning quando senha foi fornecida ("password ignored — user already exists in Firebase"); se não existir, `auth.CreateUser` com email + password.
- [x] 4.7 Persistência idempotente em transação: `INSERT INTO users (firebase_id, email, name) VALUES ($1, $2, $3) ON CONFLICT (firebase_id) DO UPDATE SET email = EXCLUDED.email, name = EXCLUDED.name RETURNING id`; depois `INSERT INTO house_members (user_id, house_id, role) VALUES (...) ON CONFLICT (user_id, house_id) DO NOTHING`.
- [x] 4.8 Tratamento de falha parcial: se `auth.CreateUser` foi bem-sucedido mas o INSERT falha, **não** chamar `auth.DeleteUser`; logar `error` com o UID criado e instruir re-execução; sair com código não-zero.
- [x] 4.9 Output final: imprimir `users.id`, `firebase_uid`, `house_id`, `role` e indicar se o Firebase user era preexistente.
- [x] 4.10 Adicionar `backend/cmd/provision-user/README.md` com exemplos de uso (`--password` e `--password-stdin`) e descrição das variáveis de ambiente requeridas.
- [x] 4.11 Rodar `go vet ./...` e `gofmt -l .`.

## 5. Mobile: exibir submitter no detalhe do cupom

- [x] 5.1 Adicionar classe `SubmittedBy { final int id; final String name; final String email; ... }` imutável com `factory SubmittedBy.fromJson` (campos snake_case) em `mobile/lib/models/receipt.dart` (mesmo arquivo do `Receipt`; o projeto não usa o layout `features/.../data/models/`).
- [x] 5.2 Adicionar campo opcional `final SubmittedBy? submittedBy` em `Receipt` e atualizar `Receipt.fromJson` para tolerar ausência (`json['submitted_by'] == null ? null : SubmittedBy.fromJson(...)`).
- [x] 5.3 Em `mobile/lib/screens/receipt_detail_screen.dart`, dentro do card do cabeçalho (logo abaixo do bloco de data/total), renderizar uma linha `Lançado por ${receipt.submittedBy!.name}` apenas quando `receipt.submittedBy != null`.
- [x] 5.4 Rodar `flutter analyze` e `flutter test`.

## 6. Testes E2E

- [x] 6.1 Em `testing/e2e/backend/tests/receipts/submitter-attribution.spec.ts`, adicionar cenário que: submete um cupom autenticado, polla até `completed`, consulta `GET /api/v1/receipts/{id}` e valida que `submitted_by` carrega `id`, `name` e `email` do usuário autenticado. (Stack E2E só autentica um usuário; o requisito da spec é "submitter aparece no detail" — coberto.)
- [x] 6.2 Adicionar cenário negativo: cupom criado com `created_by NULL` via `seedReceiptWithItems` → `GET /api/v1/receipts/{id}` retorna 200 e omite `submitted_by`.
- [x] 6.3 Adicionar cenário garantindo que `GET /api/v1/receipts` (lista) **não** retorna `submitted_by` por item, mesmo quando os receipts têm `created_by` populado.
- [x] 6.4 Rodar `npm test` na pasta `testing/e2e/backend/` com o stack isolado em pé. (25/25 passou.)

## 7. Documentação e segurança

- [x] 7.1 Atualizar `backend/scripts/README.md` (criar se não existir) ou comentário no topo de `seed_mvp.sql` esclarecendo que o seed é apenas bootstrap inicial e que membros adicionais devem ser provisionados via `cmd/provision-user`. (Atualizado o cabeçalho de `seed_mvp.sql`.)
- [x] 7.2 Garantir que `.gitignore` ignora arquivos `*.serviceaccount*.json` ou similares (Firebase service account credentials).
- [x] 7.3 Adicionar seção curta em `CLAUDE.md` (em `## Commands` ou após) com o comando `go run ./cmd/provision-user --email ... --house-name "Casa Principal" --password-stdin`.

## 8. Validação final

- [x] 8.1 `cd backend && go vet ./...` sem warnings.
- [x] 8.2 `cd backend && gofmt -l .` retorna vazio.
- [x] 8.3 `cd mobile && flutter analyze` sem erros.
- [x] 8.4 Stack E2E completo (`npm test`) verde. (25/25 em ~3min.)
- [x] 8.5 Smoke manual: criar usuário novo via CLI, autenticar pelo app com email/senha provisionados, submeter cupom, abrir detalhe e ver "Lançado por <Nome>". (Operador validou em dispositivo Galaxy S21 via WiFi adb apontando para API local em 192.168.0.139:8080.)
