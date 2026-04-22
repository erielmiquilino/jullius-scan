## Why

Hoje o worker falha terminalmente quando a SEFAZ apresenta o captcha Cloudflare (limitação aceita no MVP), o que bloqueia o fluxo real de uso — toda nota protegida por captcha fica inutilizável. O app Flutter também ainda não tem leitor de QR code funcional. Para ter um app end-to-end que cumpra o objetivo do projeto (fotografar NFC-e → persistir dados), precisamos resolver o captcha de forma confiável, e a estratégia mais robusta é devolver o desafio ao próprio usuário dentro do app (human-in-the-loop), evitando a corrida armamentista de burlar Cloudflare/Turnstile.

## What Changes

- **BREAKING** Remover a regra de falha terminal por captcha no worker: a detecção de captcha agora pausa o job ao invés de finalizá-lo como `failed`.
- Adicionar novo estado `awaiting_captcha` ao lifecycle de `ScrapingJob`, com transições `processing → awaiting_captcha → processing → completed|failed`.
- Expor endpoints novos na API:
  - `GET /api/v1/jobs/:id/captcha` — retorna a URL da SEFAZ + metadados para o app abrir o WebView.
  - `POST /api/v1/jobs/:id/captcha/resume` — recebe os cookies da sessão resolvida e reenfileira o job.
- Worker passa a serializar o estado parcial do `chromedp` (URL atual + cookies) no banco/Redis quando detecta captcha, e retoma a execução com cookies injetados quando o job volta à fila.
- Adicionar timeout configurável para o estado `awaiting_captcha` (default 10 min), após o qual o job é marcado como `failed` com motivo `captcha_timeout`.
- Implementar no app Flutter o leitor de QR code da NFC-e (câmera) e o fluxo de WebView para resolver captcha quando o job estiver em `awaiting_captcha`, com extração e envio dos cookies ao backend.
- Adicionar retry-aware: se a sessão devolvida pelo app expirar no worker (nova detecção de captcha), o job volta para `awaiting_captcha` uma vez (máx. 2 rodadas humanas) antes de falhar.
- Estender a suite E2E para cobrir o fluxo com captcha simulado pelo mock SEFAZ (nova rota de captcha + simulação de resolução).

## Capabilities

### New Capabilities
- `mobile-qrcode-capture`: leitura de QR code de NFC-e pela câmera do app Flutter e submissão da URL ao backend.
- `mobile-captcha-webview`: fluxo no app que abre um WebView com a URL SEFAZ, detecta a resolução do captcha pelo usuário, extrai os cookies da sessão autenticada e entrega ao backend.

### Modified Capabilities
- `async-receipt-scraping`: deixa de tratar captcha como falha terminal; passa a pausar o job, persistir estado e retomar com cookies fornecidos pelo app.
- `receipt-ingestion-api`: adiciona endpoints de captcha (`GET/POST /jobs/:id/captcha*`) e o novo estado `awaiting_captcha` na resposta pública do job.

## Impact

- **Backend (`backend/`)**:
  - `internal/domain/models.go` — novo valor `JobStatusAwaitingCaptcha`, campos para cookies/URL parcial no `ScrapingJob`.
  - `internal/database/queries.go` — queries para salvar/recuperar estado parcial e transicionar para `awaiting_captcha`.
  - `internal/api/handlers.go` + `router.go` — dois handlers novos (captcha info / resume) com checagem house-scoped.
  - `internal/scraper/` — detecção de Cloudflare challenge, pausa do job, injeção de cookies ao retomar.
  - `migrations/` — nova migração adicionando colunas (`captcha_pending_at`, `captcha_session_cookies`, `captcha_current_url`) e novo valor no enum/check de status.
- **Mobile (`mobile/`)**:
  - Nova dependência `mobile_scanner` (ou equivalente) para leitura de QR code.
  - Nova dependência `webview_flutter` + `webview_cookie_manager` para o WebView e extração de cookies.
  - Novas telas/controllers: `QrScannerScreen`, `CaptchaWebViewScreen`, e integração no fluxo de acompanhamento de job.
  - `lib/services/api_client.dart` — métodos para os novos endpoints de captcha.
- **E2E (`testing/e2e/backend/`)**:
  - `mocks/sefaz/` — nova página simulando challenge Cloudflare + fixtura de cookies "resolvidos".
  - Novo spec `tests/receipts/captcha-resume.spec.ts` exercitando pausar + retomar.
- **Infra**: nenhuma mudança em `deploy/` — o fluxo não introduz novos serviços, apenas endpoints.
