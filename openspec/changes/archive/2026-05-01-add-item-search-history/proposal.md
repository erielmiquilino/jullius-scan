## Why

A lista atual do app só permite ver as notas uma a uma — não há como responder perguntas cotidianas como "quanto eu paguei na última banana?" ou "onde comprei aquele detergente?". Conforme o histórico cresce, encontrar o preço/local da última compra de um item exige abrir nota por nota. O dado já está no banco (linhas de `items` ligadas a `receipts` e `stores`); falta uma busca agregada que entregue o resumo direto no resultado.

## What Changes

- Adicionar busca de itens por descrição ou código de barras, escopada por casa, com filtro temporal (padrão últimos 30 dias).
- Novo endpoint `GET /api/v1/items/search?q=<termo>&period_days=<n|null>` retornando até 50 resultados agrupados, ordenados pela última compra desc.
- Agrupamento: itens com `barcode` colapsam em uma linha por EAN; itens sem `barcode` são uma linha por descrição literal distinta.
- Auto-detecção de input: query `^\d{8,}$` busca exato em `barcode`; demais usam ILIKE com `unaccent` + trigram em `description`.
- Cada resultado retorna: descrição, EAN (se houver), `last_purchased_at`, `last_unit_price`, loja da última compra, `purchase_count` no período, `average_unit_price` ponderado e `previous_unit_price` (para variação ↑/↓), além do `receipt_id` da última compra para navegação direta.
- Habilitar extensões Postgres `pg_trgm` e `unaccent` via nova migration; índice GIN trigram em `unaccent(lower(description))` e índice em `barcode`.
- Mobile: SearchBar inline no topo do `home_screen.dart` + linha de FilterChips com `30d · 90d · 6m · 1a · Tudo`. Com `q.length>=3` e debounce 300ms a lista de notas é substituída pelos resultados de itens. Tap em um item navega para `ReceiptDetailScreen` da última compra.
- E2E: nova spec Playwright cobrindo agregação por barcode, agregação por descrição, filtro temporal, busca numérica, isolation por house e empty state.

## Capabilities

### New Capabilities
- `item-search-history`: busca agregada de itens com resumo de última compra (data, preço unitário, loja), histórico do período (qtd, média, variação) e link direto para a nota da última compra.

### Modified Capabilities
- `backend-e2e-automation`: adicionar cenários de cobertura para o novo endpoint de busca de itens.

## Impact

- **Backend**: nova migration `007_item_search_indexes` (extensões + índices), novo handler `SearchItems` em `internal/api/handlers.go`, nova query agregada em `internal/database/queries.go`, rota `GET /api/v1/items/search` em `internal/api/router.go`, novo tipo de resposta em `internal/domain/models.go` (ou em pacote `api` próprio para DTOs).
- **Mobile**: `home_screen.dart` ganha SearchBar + chips e modo de exibição alternado; novo modelo `ItemSearchResult` em `lib/models/`; novo método em `services/api_client.dart`; widget de card dedicado para o resultado.
- **E2E**: nova spec em `testing/e2e/backend/tests/items/search.spec.ts`; possível helper de seed em `tests/fixtures.ts` para pré-popular receipts/items diversos.
- **Infra**: requer Postgres com permissão de `CREATE EXTENSION` (já é o caso do bootstrap atual). Sem mudanças no `docker-compose.yml`.
- **Performance**: índice GIN trigram cresce com o nº de itens; volume é pessoal/baixo, sem risco prático.
