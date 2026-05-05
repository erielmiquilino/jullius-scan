## Context

O fluxo de captcha é dividido entre backend e mobile. O worker detecta captcha, salva URL/cookies/fase do job e muda o status para `awaiting_captcha`; o app abre a URL em WebView, espera reconhecer conteúdo NFC-e, extrai cookies/UA e chama o endpoint de resume. O incidente observado em produção falhou na segunda pausa, durante a fase `detail`: a WebView exibiu a página detalhada da SEFAZ, mas o app não reconheceu a resolução e não chamou o backend.

O backend já persiste `parsed_summary` para captchas na fase de detalhe. Isso permite concluir o job se o app enviar o HTML detalhado renderizado após a resolução, evitando depender de uma nova navegação headless que pode reacender Cloudflare.

## Goals / Non-Goals

**Goals:**

- Reconhecer automaticamente páginas NFC-e de resumo e detalhe, mesmo quando a WebView muda o DOM sem nova navegação.
- Oferecer uma ação manual segura para o usuário continuar quando o detector automático falhar.
- Enviar ao backend cookies, UA, URL atual e HTML renderizado final da WebView.
- Permitir que jobs pausados na fase `detail` sejam concluídos a partir do HTML enviado pelo app.
- Registrar erro não fatal quando o app permanece tempo demais em página SEFAZ não reconhecida.

**Non-Goals:**

- Automatizar a resolução do captcha.
- Persistir HTML bruto de notas fiscais além do necessário para concluir o resume.
- Alterar o modelo de autenticação, House ou idempotência de recibos.
- Remover o caminho atual de resume por cookies; ele continua sendo o fallback e caminho principal para captcha na fase `summary`.

## Decisions

1. **Enviar HTML no payload de resume em vez de criar um endpoint separado.** O estado pertence ao ato de retomar o captcha e deve ser validado junto com House/status do job. Alternativa considerada: endpoint `POST /captcha/page-state`; foi descartado por criar mais uma transição e mais estados intermediários.

2. **Usar HTML enviado apenas para fase `detail`.** Na fase de detalhe, o backend já tem `parsed_summary`, então o HTML detalhado só enriquece itens com EAN e permite persistir a nota. Na fase `summary`, o backend ainda precisa da navegação/parsing completo para obter dados normalizados e continuará usando cookies/UA.

3. **Não armazenar permanentemente o HTML renderizado.** O handler grava o HTML no payload do job/estado de resume apenas se necessário para o worker completar a fase; após persistir a nota, o job finaliza sem expor o HTML em responses.

4. **Detector mobile com sinais positivos e negativos.** O JS considera páginas captcha/challenge como não resolvidas, e páginas de nota/detalhe como resolvidas por sinais de DOM, URL, título e texto. Isso reduz falsos positivos em páginas de verificação.

5. **Rechecagem controlada na WebView.** Além de `onPageFinished`, a tela executa checks periódicos e injeta um `MutationObserver` que marca mudanças relevantes. O app só submete uma vez, protegido por `_isSubmitting`/`_captchaResolved`.

6. **Botão manual envia o mesmo estado final.** A ação "Já resolvi, continuar" captura cookies, UA, URL e HTML atuais; se o backend rejeitar, a tela permanece aberta com opção de retry.

## Risks / Trade-offs

- **Falso positivo no detector mobile** → Mitigado por sinais negativos explícitos de captcha e por validação backend via parser de detalhe.
- **HTML grande no payload** → Mitigado por limite de tamanho no backend e envio apenas durante resume manual/automático.
- **HTML sensível em logs** → O backend não deve logar o conteúdo; apenas tamanho e presença.
- **Worker ainda pode precisar navegar em alguns casos** → O caminho antigo por cookies permanece como fallback quando o HTML não é enviado, é inválido ou a fase não é `detail`.
- **Crashlytics não é log stream** → Registrar não fatal somente quando há suspeita de travamento, para que breadcrumbs e custom keys sejam enviados sem gerar excesso de ruído.
