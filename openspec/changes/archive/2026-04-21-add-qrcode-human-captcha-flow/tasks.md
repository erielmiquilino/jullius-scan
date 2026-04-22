## 1. Backend — schema e domínio

- [x] 1.1 Criar migração `002_captcha_resume.up.sql` adicionando colunas `captcha_current_url TEXT`, `captcha_session_cookies JSONB`, `captcha_pending_at TIMESTAMPTZ`, `captcha_resumed_at TIMESTAMPTZ`, `captcha_retry_count SMALLINT NOT NULL DEFAULT 0` em `scraping_jobs`, e atualizar a check constraint de `status` para incluir `awaiting_captcha`
- [x] 1.2 Criar migração `.down.sql` correspondente revertendo as alterações
- [x] 1.3 Adicionar `JobStatusAwaitingCaptcha JobStatus = "awaiting_captcha"` em `internal/domain/models.go`
- [x] 1.4 Adicionar campos de captcha (`CaptchaCurrentURL *string`, `CaptchaSessionCookies *json.RawMessage`, `CaptchaPendingAt *time.Time`, `CaptchaResumedAt *time.Time`, `CaptchaRetryCount int`) no struct `ScrapingJob`
- [x] 1.5 Adicionar `FailureReasonCaptchaExpired` e `FailureReasonCaptchaTimeout` em `internal/domain/models.go`

## 2. Backend — queries e fila

- [x] 2.1 Adicionar `PauseJobForCaptcha(ctx, jobID, currentURL, cookies) error` em `internal/database/queries.go` (transição atômica `processing → awaiting_captcha` com persistência de cookies/URL/timestamp)
- [x] 2.2 Adicionar `ResumeJobFromCaptcha(ctx, jobID, cookies) error` (valida que o job está em `awaiting_captcha`, incrementa `captcha_retry_count`, atualiza cookies e volta para `processing`)
- [x] 2.3 Adicionar `GetCaptchaContext(ctx, jobID) (url, userAgent, error)` devolvendo os dados necessários ao WebView
- [x] 2.4 Adicionar `ExpireAwaitingCaptchaJobs(ctx, olderThan time.Duration) error` marcando como `failed` com reason `captcha_timeout`
- [x] 2.5 Estender `internal/queue/client.go` com fila prioritária `queue:resume` lida antes de `queue:jobs` no BRPOP

## 3. Backend — API handlers

- [x] 3.1 Implementar `GET /api/v1/jobs/:id/captcha` em `internal/api/handlers.go` com checagem house-scoped, retornando `{sefaz_url, user_agent}`; 409 se o job não estiver em `awaiting_captcha`
- [x] 3.2 Implementar `POST /api/v1/jobs/:id/captcha/resume` recebendo `{cookies: [{name, value, domain, path, expires, http_only, secure, same_site}]}`, persistindo via `ResumeJobFromCaptcha` e enfileirando em `queue:resume`
- [x] 3.3 Registrar as duas rotas no `internal/api/router.go` atrás do middleware Firebase + house resolver
- [x] 3.4 Garantir que `GET /api/v1/jobs/:id` inclui o novo status `awaiting_captcha` no response (formato RFC3339 para timestamps novos, `omitempty` onde aplicável)
- [x] 3.5 Adicionar job periódico (goroutine do `cmd/api`) que chama `ExpireAwaitingCaptchaJobs` a cada 1 min com TTL configurável via env `CAPTCHA_TIMEOUT` (default 10min)

## 4. Backend — worker e scraping

- [x] 4.1 Em `internal/scraper/executor.go`, adicionar `detectCaptchaChallenge(ctx) (isChallenge bool, currentURL string, cookies []Cookie, err error)` que checa presença de marcadores Cloudflare (ex: elemento `div#cf-challenge`, título "Just a moment", ou marcador do mock)
- [x] 4.2 Substituir o caminho de falha por captcha pelo caminho de pausa: chamar `PauseJobForCaptcha` e retornar do `Execute` sem erro fatal
- [x] 4.3 Em `internal/scraper/worker.go`, implementar consumo da fila `queue:resume` com payload `{job_id, cookies}`; antes de navegar, injetar cookies no contexto chromedp via `network.SetCookies`
- [x] 4.4 Após retomar, se `detectCaptchaChallenge` ainda for positivo e `captcha_retry_count >= 1`, falhar o job com `captcha_expired`; caso contrário voltar a `awaiting_captcha`
- [x] 4.5 Padronizar o user agent usado pelo chromedp em constante exportada (`scraper.UserAgent`) consumida também pelo handler de captcha context

## 5. E2E — mock SEFAZ e testes

- [x] 5.1 Adicionar em `testing/e2e/backend/mocks/sefaz/` a rota `GET /nfce/captcha-challenge` retornando HTML com marcador detectável e `Set-Cookie` simulando sessão
- [x] 5.2 Adicionar lógica no mock: se a request vier com um cookie específico (`e2e_captcha_solved=1`), responder com o HTML da nota real; caso contrário, o HTML de challenge
- [x] 5.3 Criar helper `src/support/captcha.ts` com `fetchCaptchaContext(api, jobId, token)` e `submitCaptchaResume(api, jobId, cookies, token)`
- [x] 5.4 Criar `tests/receipts/captcha-resume.spec.ts` cobrindo: submit URL → job vira `awaiting_captcha` → GET context → POST resume com cookie mock → job completa e receipt é persistido
- [x] 5.5 Adicionar teste `tests/receipts/captcha-timeout.spec.ts` que força `CAPTCHA_TIMEOUT` curto via env do compose e valida que job expira com reason `captcha_timeout`
- [x] 5.6 Adicionar teste `tests/receipts/captcha-expired.spec.ts` que resume com cookie inválido e verifica segundo `awaiting_captcha` seguido de `failed+captcha_expired`

## 6. Mobile — dependências e QR scanner

- [x] 6.1 Adicionar em `mobile/pubspec.yaml`: `mobile_scanner: ^5.x`, `webview_flutter: ^4.x`, `webview_cookie_manager: ^2.x`, `permission_handler: ^11.x`
- [x] 6.2 Configurar permissões de câmera no `AndroidManifest.xml` e `Info.plist` (`NSCameraUsageDescription`)
- [x] 6.3 Criar `lib/features/scanner/qr_scanner_screen.dart` com `StatefulWidget` usando `MobileScanner`; validar URL com regex de NFC-e antes de submeter
- [x] 6.4 Criar `lib/features/scanner/manual_entry_dialog.dart` como fallback para colar URL manualmente
- [x] 6.5 Integrar a tela do scanner no fluxo principal (botão/FAB na home autenticada)

## 7. Mobile — API client e modelos

- [x] 7.1 Adicionar em `lib/services/api_client.dart` os métodos `fetchCaptchaContext(jobId)` e `submitCaptchaResume(jobId, cookies)`
- [x] 7.2 Criar modelo `lib/models/captcha_context.dart` (`sefazUrl`, `userAgent`) com `fromJson`
- [x] 7.3 Criar modelo `lib/models/session_cookie.dart` (snake_case keys: `name`, `value`, `domain`, `path`, `expires`, `http_only`, `secure`, `same_site`) com `toJson` compatível com o contrato do backend
- [x] 7.4 Estender enum de `JobStatus` em `lib/models/job.dart` com `awaitingCaptcha` + `fromString()` retornando `unknown` para valores futuros

## 8. Mobile — WebView de captcha

- [x] 8.1 Criar `lib/features/captcha/captcha_webview_screen.dart` recebendo `jobId`; ao montar, chama `fetchCaptchaContext` e carrega o WebView com `sefazUrl` + user agent
- [x] 8.2 Monitorar `onPageFinished` e considerar resolvido quando o HTML contiver marcador NFC-e (executar JS pequeno para checar presença de um seletor específico)
- [x] 8.3 Ao detectar resolução, extrair todos os cookies do domínio SEFAZ via `WebviewCookieManager.getCookies(sefazUrl)` e chamar `submitCaptchaResume`
- [x] 8.4 Tratar erro de `submitCaptchaResume` com snackbar/diálogo oferecendo retry ou cancelar (mantém WebView aberto)
- [x] 8.5 Expor botão "Cancelar" que retorna à tela de tracking sem alterar o job

## 9. Mobile — integração no fluxo de job

- [x] 9.1 Em `lib/features/jobs/job_tracking_screen.dart`, ao ver status `awaitingCaptcha`, navegar automaticamente (ou via CTA) para `CaptchaWebViewScreen`
- [x] 9.2 Após retorno da `CaptchaWebViewScreen` bem-sucedido, manter o polling ativo até status terminal (`completed|failed`)
- [x] 9.3 Renderizar mensagem explicativa para os possíveis `FailureReason`s novos (`captcha_expired`, `captcha_timeout`) com CTA para ressubmeter a URL

## 10. Validação final

- [x] 10.1 Rodar `cd backend && go vet ./... && gofmt -l .` com saída limpa
- [x] 10.2 Rodar `cd testing/e2e/backend && npm run env:up && npm test && npm run env:down` com todos os specs verdes (incluindo os novos de captcha)
- [x] 10.3 Rodar `cd mobile && flutter analyze && flutter test` com saída limpa
- [x] 10.4 Validar manualmente em um device Android real: scan de QR code de NFC-e → fluxo de WebView → job completa
- [x] 10.5 Atualizar `openspec/changes/add-qrcode-human-captcha-flow/tasks.md` marcando cada item como `[x]` conforme concluído
