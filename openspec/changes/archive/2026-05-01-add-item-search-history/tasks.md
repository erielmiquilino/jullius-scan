## 1. Database migration

- [x] 1.1 Criar `backend/migrations/007_item_search_indexes.up.sql` com `CREATE EXTENSION IF NOT EXISTS pg_trgm`, `CREATE EXTENSION IF NOT EXISTS unaccent`, índice GIN trigram em `unaccent(lower(description))` e índice parcial em `barcode WHERE barcode IS NOT NULL`
- [x] 1.2 Criar `backend/migrations/007_item_search_indexes.down.sql` com `DROP INDEX` correspondentes (não desinstalar extensões)
- [x] 1.3 Subir o stack local (`docker compose -f backend/tools/docker-compose.yml up -d`) e validar que `go run ./cmd/api` aplica a migration sem erro — stack já estava up; SQL aplicado via `psql` direto retornou sucesso (CREATE EXTENSION ×2, CREATE FUNCTION, CREATE INDEX ×2); `schema_migrations.version` bumpado para 7
- [x] 1.4 Inspecionar via `\d items` no psql que os dois índices novos foram criados — confirmado: `idx_items_barcode_notnull btree (barcode) WHERE barcode IS NOT NULL` e `idx_items_description_trgm gin (immutable_unaccent(lower(description::text)) gin_trgm_ops)`

## 2. Backend — query de busca agregada

- [x] 2.1 Criar DTOs `ItemSearchResponse`, `ItemSearchResult`, `ItemSearchStore`, `ItemSearchQueryEcho` — DTOs movidos para `internal/api/items.go` (junto do handler) para casar com a convenção existente (`handlers.go` co-localiza DTOs)
- [x] 2.2 Em `internal/database/queries.go`, criar `ItemQueries` com método `SearchItemsByHouse(ctx, houseID, query, periodDays *int) ([]ItemSearchAggregate, truncated bool, matchedBy string, error)` — retorna `database.ItemSearchAggregate` (em vez de `dto.ItemSearchResult`) para evitar import cycle database → api; o handler converte
- [x] 2.3 Implementar a CTE com `ROW_NUMBER() OVER (PARTITION BY COALESCE(NULLIF(barcode,''), 'desc:'||description) ORDER BY r.issued_at DESC, items.id DESC)`, agregação ponderada e `LIMIT 51` para detectar truncamento
- [x] 2.4 Branchar em barcode-exact (`^\d{8,}$` via `regexp.MustCompile`) vs descrição (`immutable_unaccent(lower(description)) ILIKE immutable_unaccent(lower($2))`) dentro do método
- [x] 2.5 Tratar `periodDays == nil` (sem filtro via `$3::int IS NULL`) e `periodDays > 0` (filtro `r.issued_at >= NOW() - make_interval(days => $3::int)`)

## 3. Backend — handler e roteamento

- [x] 3.1 Adicionar método `SearchItems(w, r)` em `*Handlers` — implementado em `internal/api/items.go` (arquivo dedicado para a feature em vez de `handlers.go`); lê `q`, `period_days`, valida e chama `database.ItemQueries.SearchItemsByHouse`
- [x] 3.2 Validações que falham retornam `400` via `respondError`; ausência de auth retorna `401` (já coberto pelo middleware) — códigos `INVALID_QUERY`, `INVALID_PERIOD_DAYS`, `NO_HOUSE`
- [x] 3.3 Registrar `r.Get("/items/search", h.SearchItems)` no grupo autenticado em `internal/api/router.go`
- [x] 3.4 Rodar `go vet ./...` e `gofmt -l .` (output vazio)

## 4. E2E — cobertura do endpoint

- [x] 4.1 Adicionar helpers de seed direto — `seedReceiptWithItems(houseId, store, items, { issuedAt })` e `seedHouse(name)` em `PostgresHelper` (`src/support/postgres.ts`); o `seedHouse` faz `setval` na sequence de `houses` antes do insert para evitar colisão com a casa id=1 do seed base
- [x] 4.2 Criar `testing/e2e/backend/tests/items/search.spec.ts` importando `{ test, expect }` de `../fixtures` — também adicionados tipos `ItemSearchResult`/`ItemSearchResponse` em `src/support/types.ts`
- [x] 4.3 Cobrir: agregação por barcode (mesmo EAN, descrições diferentes → 1 resultado, `purchase_count=2`)
- [x] 4.4 Cobrir: agregação por descrição literal (sem barcode, descrições diferentes → 2 resultados)
- [x] 4.5 Cobrir: filtro temporal padrão (60d ignorado, 5d incluído) + cenário extra de `period_days=90` que inclui ambos
- [x] 4.6 Cobrir: busca numérica com ≥8 dígitos casa exato em barcode (`matched_by=barcode`)
- [x] 4.7 Cobrir: isolation por casa (item de outra casa não retorna)
- [x] 4.8 Cobrir: query <3 chars retorna 400; `period_days` negativo retorna 400; e cenário extra sem auth retorna 401
- [x] 4.9 Cobrir: `previous_unit_price` populado quando há ≥2 compras; `null` quando há só 1
- [x] 4.10 Cobrir: `average_unit_price` ponderado (2kg@4 + 1kg@7 → 5.00) + extra: descrição diacrítico-insensitive (`acai` casa `AÇAÍ`)
- [x] 4.11 Rodar `npm run env:up && npx playwright test tests/items/search.spec.ts` — **12 passed (7.5s)**

## 5. Mobile — modelo, serviço e busca

- [x] 5.1 Criar `mobile/lib/models/item_search_result.dart` com `ItemSearchResult` (campos imutáveis, `factory fromJson`), `ItemSearchStore` e `ItemSearchPage` (envelope com `items` + `truncated`)
- [x] 5.2 Em `mobile/lib/services/api_client.dart`, adicionar `Future<ItemSearchPage> searchItems(String query, {int? periodDays})` com decode do envelope; envia `period_days=null` quando o usuário escolhe "Tudo"
- [x] 5.3 400 do backend lançado como `ApiError` com a mensagem do servidor — coberto automaticamente por `_handleResponse`

## 6. Mobile — UI da busca no Home

- [x] 6.1 Em `mobile/lib/screens/home_screen.dart`, adicionar estado `_query`, `_periodDays` (default 30), `_searchResults`, `_searchTruncated`, `_searchLoading`, `_searchError`, `_debounceTimer`, `_searchController`, `_searchRequestId` (descarta respostas obsoletas)
- [x] 6.2 Adicionar `_SearchHeader` widget no topo do `body` com `TextField` (placeholder "Buscar item ou código de barras", botão limpar) e `Wrap` de `FilterChip` para `30d · 90d · 6m · 1a · Tudo`
- [x] 6.3 Implementar `_onQueryChanged` com debounce de 300ms; disparar `_runSearch()` apenas quando `q.trim().length >= 3`; cancelar timer quando query encurta
- [x] 6.4 Implementar `_runSearch()` chamando `apiClient.searchItems` e atualizando estado; tratar `ApiError` e fallback genérico
- [x] 6.5 No `_buildBody`, alternar entre `_buildReceiptsList` (q vazio/curto) e `_buildSearchResults` (q ≥ 3 chars)
- [x] 6.6 Criar widget `_ItemSearchResultCard` exibindo descrição, EAN (quando houver), última data `dd/MM/yyyy`, último preço unitário em R$, nome da loja, contador de compras, preço médio e `_PriceVariationChip` (↓ verde se menor, ↑ vermelho se maior, `=`/`—` neutro se igual ou `previous_unit_price` null)
- [x] 6.7 Tap no card → `Navigator.push` para `ReceiptDetailScreen(apiClient: ..., receiptId: result.receiptId)` via `_navigateToReceiptById`
- [x] 6.8 Quando `_searchTruncated` for true, exibir banner discreto no topo da lista: "Mostrando 50 — refine a busca para ver mais"
- [x] 6.9 Rodar `flutter analyze` e `flutter test` — analyzer limpo (No issues found! 15.2s); 3 widget tests existentes seguem verdes
- [ ] 6.10 Subir o app via `flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080`, validar fluxo completo (digitar 3 chars, ver resultados, alternar chips, tocar item, voltar) — **dependente de device físico/emulador, deixado para o usuário rodar**

## 7. Validação final

- [x] 7.1 `openspec validate add-item-search-history --strict` — output: `Change 'add-item-search-history' is valid`
- [x] 7.2 Rodar `go vet ./...` e `gofmt -l .` no estado final — sem output em ambos (limpo)
- [x] 7.3 Rodar `npx playwright test` (suite completa verde) — **22 passed (3.7m)** incluindo as 12 novas + 10 existentes
- [ ] 7.4 Smoke manual no app contra backend local com seed de 3-4 receipts variadas — **dependente de device físico/emulador, deixado para o usuário rodar**
