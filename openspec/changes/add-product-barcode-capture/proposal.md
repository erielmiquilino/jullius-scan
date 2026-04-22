## Why

O objetivo central do Jullius Scan é construir uma base de dados pessoal para consulta de preços de itens de supermercado — e essa consulta só é útil se cada item puder ser identificado de forma estável por seu código de barras (EAN). A página resumo da NFC-e que raspamos hoje (`NFCe_Detalhes.aspx`) não expõe o EAN; o código só aparece na página de **Consulta detalhada** (`Nfe_DetalheCert.aspx`), acessível via botão "Ver NFC-e detalhada". Sem essa informação, dois itens com a mesma descrição em notas diferentes não podem ser consolidados como o mesmo produto, inviabilizando o caso de uso principal.

## What Changes

- Estender o worker de scraping para, após extrair a página resumo, navegar até a página de Consulta detalhada e capturar os códigos EAN (Comercial e Tributável) de cada item.
- Adicionar um segundo ponto de detecção de captcha: quando a navegação entre resumo → detalhe dispara desafio (às vezes um Cloudflare Turnstile entre as páginas), o job deve pausar em `awaiting_captcha` reaproveitando o fluxo WebView existente, reabrindo na URL da página detalhada.
- Ao retomar o job após captcha, continuar a partir da página detalhada — não reiniciar do zero a partir da URL fiscal original (o comportamento atual).
- Persistir o EAN no modelo de item (nova coluna `barcode TEXT NULL` em `receipt_items`) e expor em todas as respostas de API que retornam itens.
- Mapear o campo `barcode` no modelo Dart de `ReceiptItem` e exibi-lo na tela de detalhe do recibo quando presente.
- **BREAKING (parsing)**: O parser de itens passa a depender da página detalhada como fonte autoritativa de EAN. Itens sem EAN na página detalhada (serviços, produtos sem cadastro GTIN) permanecem persistidos com `barcode = NULL` — comportamento compatível, mas o schema do JSON de resposta ganha um campo opcional.

## Capabilities

### New Capabilities
- Nenhuma nova capability — o escopo é extensão das capabilities existentes de scraping e resposta de API.

### Modified Capabilities
- `async-receipt-scraping`: o ciclo de scraping passa a ter duas fases (resumo → detalhe), com um segundo possível ponto de pausa por captcha; a persistência de itens passa a incluir `barcode`.
- `receipt-ingestion-api`: o payload de resposta de receipt/itens passa a incluir o campo opcional `barcode`.
- `mobile-captcha-webview`: o fluxo WebView deve suportar reabertura em uma URL arbitrária persistida no job (a URL da página detalhada), não apenas a URL fiscal original.

## Impact

- **Código afetado (backend):**
  - `backend/internal/scraper/executor.go`: nova ação `ClickDetailLink` ou navegação direta para `Nfe_DetalheCert.aspx`; detecção de captcha reutilizada na segunda página.
  - `backend/internal/scraper/parser.go`: novo parser `parseDetailPage` para extrair EAN de cada item (via regex sobre os blocos `<Código EAN Comercial>` ou parseamento do XML embutido, se disponível).
  - `backend/internal/scraper/worker.go`: orquestrar as duas fases; na pausa, persistir a URL da página detalhada em `captcha_current_url`.
  - `backend/internal/domain/models.go`: campo `Barcode *string` em `ReceiptItem`.
  - `backend/internal/database/queries.go`: `INSERT` e `SELECT` de itens incluem `barcode`.
  - Nova migration `005_item_barcode.up.sql` / `.down.sql` para adicionar `barcode TEXT` em `receipt_items`.
- **Código afetado (mobile):**
  - `mobile/lib/models/receipt.dart`: campo `barcode` em `ReceiptItem`.
  - `mobile/lib/screens/receipt_detail_screen.dart`: exibir o EAN quando presente.
- **Testes E2E:** o mock SEFAZ precisa servir também a rota `Nfe_DetalheCert.aspx` com fixtures que exponham o EAN; um novo cenário cobre o caso de captcha na transição resumo→detalhe.
- **Sem mudanças em API surface públicas** além do campo opcional `barcode` em respostas de itens.
