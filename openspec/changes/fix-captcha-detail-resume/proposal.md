## Why

O fluxo atual de captcha pode ficar preso quando a SEFAZ libera a página detalhada da NFC-e dentro da WebView, mas o app não reconhece automaticamente que o conteúdo final já está visível. Quando isso acontece, o backend permanece em `awaiting_captcha` até expirar, mesmo com o usuário tendo concluído o desafio.

## What Changes

- Tornar a detecção mobile de conteúdo NFC-e mais robusta para páginas de resumo e detalhe da SEFAZ/SC.
- Adicionar rechecagem automática enquanto a WebView estiver aberta, incluindo detecção de mudanças no DOM sem nova navegação.
- Adicionar uma ação manual "Já resolvi, continuar" para enviar a sessão ao backend quando a detecção automática falhar.
- Estender o endpoint de resume de captcha para aceitar URL atual e HTML final da WebView.
- Permitir que o worker conclua jobs pausados na fase `detail` usando o HTML detalhado enviado pelo app, sem depender de nova navegação headless.
- Registrar telemetria não fatal quando a WebView parece ficar presa em uma página SEFAZ não reconhecida.

## Capabilities

### New Capabilities

- Nenhuma.

### Modified Capabilities

- `mobile-captcha-webview`: melhora a detecção de captcha resolvido, adiciona fallback manual e telemetria de travamento.
- `receipt-ingestion-api`: amplia o contrato do resume de captcha para receber estado final da WebView.
- `async-receipt-scraping`: permite concluir a fase de detalhe a partir do HTML capturado no celular.

## Impact

- `mobile/lib/features/captcha/captcha_webview_screen.dart`
- `mobile/lib/services/api_client.dart`
- `mobile/lib/models/session_cookie.dart` e modelos relacionados, se necessário
- `backend/internal/api/` handlers de captcha
- `backend/internal/domain/`, `backend/internal/database/queries.go` e migrações para armazenar HTML/URL enviados no resume
- `backend/internal/scraper/worker.go` para concluir detalhe com HTML capturado
- Testes E2E backend de captcha/detail resume e análise/testes Flutter
