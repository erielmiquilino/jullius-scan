## Context

Hoje o worker de scraping navega até a URL fiscal da NFC-e (`NFCe_Detalhes.aspx`), detecta captcha, e — quando consegue renderizar — extrai descrição, quantidade, unidade e valor dos itens via regex SC-específico sobre o HTML. Essa página resumo **não expõe o EAN** (código de barras) do produto; ele só está presente na página de Consulta detalhada (`Nfe_DetalheCert.aspx`), acessível pelo botão "Ver NFC-e detalhada".

A página detalhada pertence ao mesmo domínio SEFAZ SC, porém renderiza um documento fiscal completo (tabular) e ocasionalmente dispara um desafio **Cloudflare Turnstile** entre a página resumo e a detalhada (diferente do captcha próprio do SEFAZ que já tratamos). A arquitetura atual de captcha (pause → WebView → cookies → resume) foi construída assumindo um único ponto de desafio, sempre na URL fiscal original.

O caminho entre a página resumo e a detalhada é feito por um link no botão "Ver NFC-e detalhada" que aponta para `Nfe_DetalheCert.aspx?rq=<TOKEN>`. O token na querystring é sessão-dependente — não podemos pré-computar a URL da detalhada a partir da URL fiscal original; é necessário extraí-la da própria página resumo renderizada.

Consumidor final do `barcode`: o app Flutter, que hoje já exibe itens na tela de detalhe do recibo e precisará mostrar o EAN quando disponível.

## Goals / Non-Goals

**Goals:**
- Capturar `EAN Comercial` e persistir como `barcode` em cada item da nota quando o EAN existir na página detalhada.
- Preservar o comportamento atual (item com descrição/qtd/unidade/valor) quando a navegação para a detalhada falhar, for inviável ou o item não tiver EAN cadastrado.
- Suportar captcha em **qualquer das duas fases** (resumo ou transição resumo→detalhe) reutilizando o fluxo WebView existente, inclusive retomando a navegação a partir da URL correta.
- Idempotência: retomar um job pausado na fase detalhe **não deve re-raspar a página resumo** nem re-executar o parse do resumo.

**Non-Goals:**
- Unificar/deduplicar produtos entre notas com base no EAN (isso é um follow-up de produto, não desta mudança).
- Capturar demais campos fiscais da página detalhada (NCM, CFOP, ICMS etc.) — escopo restrito ao EAN.
- Suportar SEFAZ de outros estados. A página detalhada varia por UF; este design é específico para SC e segue o padrão já estabelecido no parser.
- Cache persistente de EAN por descrição/loja; a extração é sempre feita por nota.

## Decisions

### D1. O worker executa a segunda navegação (resumo → detalhe) na mesma sessão chromedp

**Decisão:** Após parsear a página resumo com sucesso, antes de persistir os itens, o worker clica no link "Ver NFC-e detalhada" (ou navega diretamente à URL href extraída do botão) **usando o mesmo `chromedp.Context` já aberto**. Em seguida, espera a renderização, detecta captcha, e — se passar — parseia a detalhada para extrair EANs. Depois fecha o browser e persiste o merge (itens do resumo + EANs da detalhada).

**Alternativas consideradas:**
- *HTTP cru (sem browser) para a página detalhada*: o token na querystring só é válido dentro da sessão do browser que renderizou a resumo; replicar a sessão sem chromedp teria alta taxa de falha por Cloudflare.
- *Dois jobs separados (um por página)*: duplicaria tracking, custo de browser e captcha handling.

**Por quê:** Manter uma única sessão minimiza custo de inicialização do browser, maximiza probabilidade de passar por Cloudflare (cookies já aquecidos na sessão), e a chain resumo→detalhe é atômica do ponto de vista do usuário.

### D2. Captcha na fase detalhe reaproveita `awaiting_captcha` com URL da página detalhada

**Decisão:** Adicionar um campo novo **não** é necessário — o `captcha_current_url` já existe. A mudança é semântica: esse campo passa a carregar **a URL onde o captcha foi disparado**, seja ela a URL fiscal original (fase resumo) seja a URL da página detalhada (fase transição). O worker, ao retomar, usa essa URL como ponto de entrada do browser (com cookies injetados), **não** a `fiscal_url` do job.

**Correção de bug latente:** Hoje o worker retoma sempre em `msg.FiscalURL` porque a `CaptchaCurrentURL` capturada pelo SEFAZ era a `SecurityVerify.aspx` (que re-dispara captcha). Com a página detalhada, a URL persistida é a que queremos navegar. Precisamos, portanto, introduzir uma **flag de fase** (`captcha_phase = 'summary' | 'detail'`) para que o resume saiba:
- `summary` → navegar em `fiscal_url` (como hoje, ignorando `captcha_current_url` que é a página de challenge)
- `detail` → navegar em `captcha_current_url` (a URL da detalhada) e **pular** o parseamento da resumo

**Alternativa considerada:** usar apenas `captcha_current_url` sem flag de fase e heurística de hostname/path. Rejeitado por frágil — preferimos um enum explícito.

### D3. Persistência de estado intermediário entre resumo e detalhe

**Decisão:** Quando o worker pausa na transição resumo→detalhe, os itens já parseados da resumo são **persistidos em memória** do worker enquanto o job aguarda captcha, o que significa: no retomar, o worker não re-navega à resumo. Para que isso funcione após o processo ser reiniciado, os dados parciais precisam estar no PostgreSQL.

Adicionamos uma coluna `parsed_summary JSONB NULL` em `scraping_jobs` contendo o payload estruturado (store + receipt + items sem barcode) capturado antes da pausa. Ao retomar um job com `captcha_phase='detail'` e `parsed_summary IS NOT NULL`, o worker pula a resumo, navega direto à detalhada, parseia EANs, faz o merge pelo índice do item (ordem posicional) e persiste.

**Alternativa considerada:** re-raspar a resumo no retorno. Rejeitado porque (a) pagamos dois custos de captcha em vez de um, (b) a URL fiscal original pode estar propensa a ela mesma exigir captcha, criando loop.

### D4. Merge resumo↔detalhada por índice posicional, não por descrição

**Decisão:** A página detalhada lista os itens na **mesma ordem** da resumo (enumerados por `Num.` 1..N). O merge é feito por índice, não por string-matching de descrição (que tem variações de whitespace, truncamento, caracteres). Se o número de itens divergir entre as duas páginas, loga `slog.Warn` e persiste sem barcode em todos os itens da nota — é melhor perder o EAN de uma nota do que associar incorretamente.

### D5. EAN vazio / ausente é `NULL`, não string vazia

**Decisão:** `receipt_items.barcode` é `TEXT NULL`. Produtos sem EAN na detalhada (serviços, variáveis sem GTIN) persistem `NULL`. API retorna `"barcode": null` (com `omitempty` no JSON tag, o campo não aparece). O modelo Dart trata `null` como "EAN não disponível" na UI.

### D6. Não introduzir flag de feature — é mudança direta

Seguindo o princípio do CLAUDE.md ("não adicione feature flags ou shims quando pode simplesmente mudar o código"): o worker passa a sempre tentar a fase detalhada. Se a detalhada falhar por motivo não-captcha (timeout, parse), o worker loga warning, persiste os itens **sem** barcode e marca o job como `completed` (não `failed`) — a nota ainda é útil.

## Risks / Trade-offs

- **[Risco] A página detalhada pode ter layout diferente entre emitentes ou versões**
  → Mitigação: o parser da detalhada usa dois seletores em paralelo (`Código EAN Comercial` e `Código EAN Tributável`), com fallback para o primeiro match válido; fixtures E2E cobrem pelo menos um emitente; em caso de parse falho, degradação é graciosa (sem barcode, nota persiste).

- **[Risco] Captcha Cloudflare na transição pode ocorrer com muito mais frequência do que o captcha SEFAZ da fase 1**
  → Mitigação: o fluxo WebView já existe e foi validado. O custo adicional é UX (usuário precisa resolver captcha mais vezes), não correção. Monitoramos via `slog` a taxa de pausas em fase `detail` para reavaliar se precisamos de alguma estratégia de warm-up de sessão.

- **[Risco] Jobs antigos em banco (schema anterior) sem `parsed_summary` nem `captcha_phase`**
  → Mitigação: a migration adiciona colunas nullable; jobs que estavam em `awaiting_captcha` no momento do deploy retomarão na fase `summary` (default/null → summary), comportamento idêntico ao atual. Documentado na migration.

- **[Trade-off] Aumento do tempo médio de scraping (segunda navegação + parse)**
  → Estimamos +3-5s por nota quando não há captcha. Como o worker é pool de 1 job por vez e o caso de uso não é tempo-real, aceitável.

- **[Risco] Ordem posicional diverge entre resumo e detalhada**
  → Mitigação: validação explícita por contagem antes do merge; em caso de divergência, degradação graciosa (todos `barcode = NULL`) com log de warning para investigação.

- **[Risco] Parser da detalhada pode ser mais lento ou custoso devido ao tamanho maior do HTML**
  → Mitigação: regex é linear sobre o HTML; o parsing da detalhada não usa fluxo recursivo. Medimos em teste E2E; se ultrapassar 2s, investigamos.

## Migration Plan

1. Deploy do backend com migration `005_item_barcode.up.sql` (adiciona `barcode` em `receipt_items`) e `006_job_detail_phase.up.sql` (adiciona `captcha_phase` e `parsed_summary` em `scraping_jobs`).
2. Jobs em `awaiting_captcha` no momento do deploy têm `captcha_phase = NULL` → worker trata como `'summary'` (comportamento atual).
3. Jobs existentes em `completed` não são afetados — `barcode` em itens antigos permanece `NULL` (usuário pode deletar e re-scannear para obter EAN).
4. Rollback: as migrations `.down.sql` removem as colunas. Itens recém-cadastrados perdem o campo `barcode`, mas nenhum dado essencial é perdido.

## Open Questions

- A URL do botão "Ver NFC-e detalhada" é gerada server-side (SEFAZ) a partir da sessão atual — ela tem TTL? Se a sessão expirar entre resumo e detalhe, precisamos de retry? → Investigar durante implementação; se TTL for curto, o retry já existente cobre o caso.
- Vale também capturar `Código do Produto` (SKU interno do emitente, visível na detalhada) como fallback quando não há EAN? → Fora de escopo desta mudança, mas é um bom follow-up.
