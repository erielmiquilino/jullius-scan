## Context

Hoje o fluxo é: app submete URL → API enfileira job → worker com `chromedp` tenta carregar a página da SEFAZ. Quando a página retorna o desafio Cloudflare (Turnstile/interstitial), o worker não tem como resolver e o job é marcado como `failed` com `reason=captcha`. Na prática isso inviabiliza o uso real do app, porque parte significativa das URLs NFC-e cai nesse caminho.

O app Flutter ainda não tem leitura de QR code implementada — hoje a única entrada é passar URL manualmente via teste. Para fechar o ciclo end-to-end, precisamos simultaneamente:
1. Leitura de QR code no mobile.
2. Ponte humana para resolver o captcha quando o worker tropeçar.
3. Retomada do scraping com a sessão já validada pelo usuário.

Constraints relevantes:
- Uma única `House` por usuário (MVP); todo job pertence a uma casa.
- Scraping é assíncrono, worker é stateless entre jobs, dispatch via Redis BRPOP.
- O `chromedp` roda em Debian slim + Chromium no container do worker; a sessão não persiste entre execuções do worker.
- O app usa `StatefulWidget` + `setState` (sem lib de estado externa).

## Goals / Non-Goals

**Goals:**
- App Flutter lê QR code de NFC-e pela câmera e submete ao backend.
- Worker detecta desafio Cloudflare e pausa o job ao invés de falhar.
- App assume o WebView com a URL SEFAZ, usuário resolve captcha, cookies da sessão validada são devolvidos ao backend.
- Worker retoma a mesma execução com os cookies injetados e completa o scraping normalmente.
- Suite E2E cobre o caminho com captcha simulado via mock SEFAZ.

**Non-Goals:**
- Resolver captcha automaticamente (2captcha, ML, etc.).
- Suportar múltiplos usuários resolvendo captchas concorrentes para a mesma casa (MVP mantém 1:1).
- Persistir a sessão validada entre múltiplos jobs (cada job pede sua própria resolução se necessário).
- Implementar notificação push em produção — o app segue com polling ativo (já é o padrão atual).
- Migrar state management do Flutter para Provider/Riverpod.

## Decisions

### Decisão 1: Novo estado `awaiting_captcha` no lifecycle
Adicionar um estado intermediário entre `processing` e `completed|failed`, ao invés de reutilizar `processing` com uma flag lateral.

**Por quê:** o lifecycle de `JobStatus` é hoje a fonte canônica de estado para API e app. Introduzir um estado explícito (a) deixa o contrato público auto-documentado, (b) permite ao app decidir quando abrir o WebView apenas olhando o status, (c) viabiliza auditoria/métricas de quantos jobs exigiram intervenção humana.

**Alternativa considerada:** manter `processing` e adicionar campo `needs_captcha bool`. Rejeitada — aumenta o acoplamento entre API e cliente (cliente passa a ter que olhar dois campos) e complica a lógica de timeout.

### Decisão 2: Pausa síncrona do worker com persistência de cookies + URL
Quando o worker detecta o challenge, ele:
1. Captura a URL atual (pode ser diferente da original — Cloudflare redireciona).
2. Captura os cookies já acumulados pelo chromedp.
3. Persiste ambos em colunas novas na tabela `scraping_jobs` (`captcha_current_url`, `captcha_session_cookies` JSONB, `captcha_pending_at`).
4. Atualiza `status` para `awaiting_captcha` e **encerra a execução** do worker para aquele job (libera o Chromium).

Quando o job for retomado via `POST /jobs/:id/captcha/resume`, a API reenvia ele para a fila Redis com um flag de "resume". O worker, ao consumir, se detectar esse flag, inicia o chromedp já carregando os cookies do corpo da request antes de navegar para a URL persistida.

**Por quê:** manter o worker vivo esperando o captcha gastaria um slot de Chromium por minutos. Stateless resume é mais robusto e se alinha ao padrão atual (workers efêmeros, dispatch via Redis).

**Alternativa considerada:** manter o processo chromedp suspenso aguardando sinal. Rejeitada — Chromium segura RAM demais, e qualquer restart de worker (deploy) mataria todos os jobs em `awaiting_captcha`.

### Decisão 3: Cookies trafegam como JSONB, não criptografados em nível de aplicação
Os cookies de sessão Cloudflare (incluindo `cf_clearance`) serão armazenados em `captcha_session_cookies JSONB NOT NULL` e trafegados via HTTPS (Traefik). Nenhuma camada extra de criptografia no app ou backend.

**Por quê:** o canal já é TLS, o banco só é acessível via rede interna do VPS, e os cookies têm TTL curto (tipicamente 30min–2h do `cf_clearance`). A complexidade de KMS/gerenciamento de chave não se paga nesse perfil de app pessoal.

**Trade-off:** um ataque que comprometa leitura do banco pega sessões Cloudflare — aceita o risco dado o contexto de uso pessoal.

### Decisão 4: Detecção do captcha resolvido no WebView via navegação
O app não tenta "interpretar" se o captcha foi resolvido. Em vez disso, monitora `onPageFinished` e considera resolvido quando a URL atual do WebView **contém o conteúdo NFC-e esperado** (ex: presença do elemento `#tabResult` ou title contendo "NFC-e") — heurística já dominada pelo parser do backend.

**Por quê:** Cloudflare não expõe evento de "challenge passed"; a única sinal universal é a página destino finalmente renderizar.

**Alternativa considerada:** confiar apenas na mudança de URL. Rejeitada — Cloudflare às vezes mantém a mesma URL após passar o challenge.

### Decisão 5: Retry-aware para sessões expiradas
Se o worker retomar com cookies e o Cloudflare ainda assim devolver challenge (cookie expirou entre "usuário resolveu" e "worker consumiu"), o job volta a `awaiting_captcha` uma única vez. A segunda falha é terminal com `reason=captcha_expired`.

**Por quê:** evita loops infinitos humano↔worker sem pedir ao usuário que resolva ad eternum.

### Decisão 6: Leitura de QR code no Flutter via `mobile_scanner`
Usar `mobile_scanner` (^5.x) em vez de `qr_code_scanner` (deprecated) ou `flutter_barcode_scanner` (sem manutenção ativa).

**Por quê:** `mobile_scanner` é o mais mantido, suporta iOS + Android com plugin unificado (MLKit), e aceita callbacks de detecção em streaming — importante pra UX de mira automática.

### Decisão 7: WebView + extração de cookies via `webview_flutter` + `webview_cookie_manager`
`webview_flutter` é oficial da equipe Flutter e já tem `WebViewCookieManager`. Para extração confiável de **todos** os cookies (incluindo `HttpOnly`), usar o plugin complementar `webview_cookie_manager` (lê do CookieManager nativo Android/iOS em vez da JS API, que não enxerga HttpOnly).

**Por quê:** cookies Cloudflare são frequentemente marcados `HttpOnly` — ler via `document.cookie` dentro do WebView não bastaria.

## Risks / Trade-offs

- **[Risco]** Cookies Cloudflare podem expirar antes do worker retomar → **Mitigação:** endpoint de resume marca `captcha_resumed_at` e dispara push imediato do job pra fila; worker prioriza (BRPOP em lista dedicada `queue:resume` lida antes da `queue:jobs`).
- **[Risco]** User-agent do WebView ≠ user-agent do chromedp → sessão não é aceita pelo Cloudflare → **Mitigação:** fixar o mesmo UA no chromedp e no WebView (`flutter_user_agent` override); validado por teste E2E que injeta cookies gerados pelo mock com validação de UA.
- **[Risco]** Timeout de `awaiting_captcha` pega usuário que fechou o app → job nunca é retomado → **Mitigação:** timeout configurável (default 10min), motivo `captcha_timeout`; app exibe estado claro e permite ressubmeter a URL (novo job).
- **[Risco]** Tela do WebView em iOS com ITP (Intelligent Tracking Prevention) descartando cookies → **Mitigação:** usar `WKWebView` com `WKWebsiteDataStore.default()` e testar em device real durante a task de validação mobile; documentar como known limitation se persistir.
- **[Trade-off]** Guardar cookies de sessão no banco expande superfície de dados sensíveis → aceito pelo perfil de uso pessoal; TTL curto limita janela.
- **[Trade-off]** Usuário tem que interagir para resolver captcha → UX menos mágica, mas 100% confiável vs. soluções de bypass.

## Migration Plan

1. **Migration SQL** (`002_captcha_resume.up.sql`):
   - Adiciona colunas `captcha_current_url TEXT`, `captcha_session_cookies JSONB`, `captcha_pending_at TIMESTAMPTZ`, `captcha_resumed_at TIMESTAMPTZ`, `captcha_retry_count SMALLINT NOT NULL DEFAULT 0`.
   - Atualiza check constraint de `status` para incluir `'awaiting_captcha'`.
   - Script `.down.sql` reverte em ordem reversa.
2. **Deploy ordem**: migração → API + worker juntos (CI/CD atual já builda e deploya ambos). Não há breaking change de API pública antes do app novo estar em circulação (app antigo continua tratando `failed+reason=captcha` como hoje).
3. **Mobile**: versão nova do app é opt-in — usuário atualiza e só então vê fluxo de WebView. App antigo continua funcional para casos sem captcha.
4. **Rollback**: reverter imagem do backend para tag anterior + aplicar `.down.sql`. Jobs em `awaiting_captcha` no momento do rollback precisam ser movidos manualmente para `failed` (script SQL ad-hoc).

## Open Questions

- Qual o TTL ideal do `awaiting_captcha`? Proposto 10min; validar na prática com uso real.
- Precisamos de um endpoint `DELETE /jobs/:id/captcha` para o usuário cancelar explicitamente? Ou basta o timeout? → **Decisão preliminar:** só timeout no MVP; adicionar cancelamento explícito se surgir dor.
- O mock SEFAZ precisa simular o desafio Cloudflare real ou basta uma página dummy com `?captcha=1` na URL? → **Decisão preliminar:** página dummy que o worker detecta por um marcador HTML, simplifica o teste sem depender de Cloudflare real.
