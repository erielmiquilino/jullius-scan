## Context

Hoje as casas têm dezenas de notas e centenas de linhas em `items`, mas a única forma de revisitar dados é abrir cada nota em `ReceiptDetailScreen`. O schema já carrega tudo que precisamos para uma busca: `items` (description, barcode, unit_price, quantity, total_price) → `receipts` (issued_at, store_id) → `stores`. A migration `005_item_barcode` adicionou `items.barcode` (TEXT NULL) e a feature de captura de EAN na consulta detalhada acabou de aterrissar (`e5be2f7`), o que tornou viável agrupar por código de barras quando ele está presente. O endpoint precisa ser house-scoped via `middleware.ResolveHouse`, idêntico aos demais.

A SEFAZ devolve descrições inconsistentes (variação de capitalização, sufixos, abreviações) e o app é de uso pessoal — volume é baixo (centenas a poucos milhares de items por casa) mas crescimento é monotônico. Acentos e variações de capitalização precisam ser tolerados; full-text com tsvector seria overkill.

## Goals / Non-Goals

**Goals:**
- Endpoint único `GET /api/v1/items/search` que devolva resultados agregados prontos para renderizar.
- Tolerância a acentos e capitalização na busca por descrição.
- Detecção automática de busca por barcode (sem toggle de UI).
- Janela temporal padrão de 30 dias com opção de ampliar.
- Mobile entrega busca em tempo real com debounce (300ms, mínimo 3 chars).
- Tap no resultado abre direto a nota da última compra.

**Non-Goals:**
- Não criar uma tabela de "produto canônico" nem normalização semântica de descrições — agrupamento se baseia em barcode quando existe; sem barcode, descrição literal.
- Não implementar histórico completo de compras do item em tela própria (clique vai direto pra nota da última compra).
- Não implementar paginação clássica — limite fixo 50 com flag `truncated` é suficiente para o uso pessoal.
- Não adicionar full-text search com dicionário pt-BR; pg_trgm + unaccent cobre o caso.
- Não retroagir nem renormalizar dados existentes — ajustes são de schema (extensões + índices) e leitura.

## Decisions

### Decisão 1 — Postgres: extensões e índices

**Escolha:** Habilitar `pg_trgm` e `unaccent` via migration nova (`007_item_search_indexes`) e criar dois índices em `items`:
1. `CREATE INDEX idx_items_description_trgm ON items USING GIN (unaccent(lower(description)) gin_trgm_ops);`
2. `CREATE INDEX idx_items_barcode_notnull ON items (barcode) WHERE barcode IS NOT NULL;`

**Por quê:** `pg_trgm` torna `ILIKE '%termo%'` indexado e barato; `unaccent` permite tratar "açaí"/"acai" como equivalentes. A migration tenta `CREATE EXTENSION IF NOT EXISTS` — em Postgres oficial isso requer superuser, e já é o caso do compose local e da imagem em produção. Se o ambiente não permitir, a migration falha com mensagem clara e a feature precisa ser revisitada.

**Alternativas descartadas:**
- ILIKE simples sem unaccent: descartado por usuário ("acai" deveria casar "AÇAÍ").
- tsvector com dicionário pt-BR: overkill para o volume; perde matches parciais ("ban" não casaria "banana") e exige stemming que pode produzir resultados estranhos em descrições truncadas.
- Coluna gerada `description_normalized`: mais escrita por insert, índice equivalente em utilidade ao functional index. Funcional é mais simples.

### Decisão 2 — Detecção de busca por barcode

**Escolha:** Considerar `q` como busca exata em barcode quando `q` casar com `^\d{8,}$` após `strings.TrimSpace`. Caso contrário, ILIKE em descrição.

**Por quê:** EAN-8/EAN-13 são respectivamente 8 e 13 dígitos; números menores quase nunca aparecem em descrições e esse threshold elimina falsos positivos. Sem toggle: usuário não precisa pensar.

**Alternativas descartadas:**
- Buscar em ambos via OR: o branch ILIKE em descrição é desnecessário quando a query é claramente um código; complica a query e diminui performance.
- Toggle de UI: friction sem retorno em app pessoal.

### Decisão 3 — Agrupamento e SQL

**Escolha:** Single query com CTE que materializa as linhas elegíveis (filtro de casa + período + match) e janela `ROW_NUMBER() OVER (PARTITION BY group_key ORDER BY r.issued_at DESC, items.id DESC)` para identificar a última compra. `group_key` é `COALESCE(NULLIF(barcode, ''), 'desc:' || description)`.

**Por quê:** Agrupar por barcode quando presente e por descrição quando ausente é exatamente o comportamento pedido. `'desc:'` prefixa para garantir que descrições não colidam com barcodes (improvável, mas barato). `ORDER BY r.issued_at DESC, items.id DESC` desempata determinísticamente quando duas notas têm o mesmo timestamp. A última compra define `description` exibida e `store`.

Cálculos por grupo (em segunda CTE / agregação):
- `purchase_count = COUNT(*)`
- `average_unit_price = SUM(total_price) / NULLIF(SUM(quantity), 0)` (média ponderada — financeiramente correta, evita divisão por zero)
- `previous_unit_price = unit_price WHERE row_num = 2` (NULL se não existir)

LIMIT 50 aplicado depois de ordenar grupos por `last_purchased_at DESC`. Para detectar truncamento, executa a mesma query agregada com `LIMIT 51` e devolve `truncated=true` se vierem 51 linhas (descartando a 51ª).

**Alternativas descartadas:**
- Múltiplas queries (uma para grupos, outra para penúltima): mais round-trips, mais lock-overhead. CTE única com window functions é trivial em pg.
- COUNT(*) OVER () para totais: desnecessário; só precisamos de truncated boolean.

### Decisão 4 — Forma do endpoint e DTO

**Escolha:** `GET /api/v1/items/search?q=<termo>&period_days=<n>` com resposta:
```json
{
  "items": [{
    "description": "BANANA NANICA",
    "barcode": "7891234567890",
    "last_purchased_at": "2026-04-25T18:30:00Z",
    "last_unit_price": 4.50,
    "last_total_price": 9.00,
    "previous_unit_price": 4.20,
    "average_unit_price": 4.38,
    "purchase_count": 3,
    "store": { "id": 7, "name": "Mercado X", "cnpj": "11.222.333/0001-44" },
    "receipt_id": 142
  }],
  "truncated": false,
  "query": { "q": "banana", "period_days": 30, "matched_by": "description" }
}
```

`matched_by` é `"barcode"` ou `"description"` — útil para o cliente decidir se faz sentido destacar o EAN na UI e para debug. Sem `period_days` na URL → padrão 30. `period_days` com valor `0` ou `null` (string literal) → todo o histórico. `q` ausente ou < 3 chars (após trim) → 400.

DTOs ficam num pacote dedicado `internal/api/dto` (novo) para não poluir `internal/domain` com wire types.

**Alternativas descartadas:**
- POST com body JSON: GET é mais natural para query sem efeitos colaterais.
- Devolver lista flat sem envelope: `truncated` e `query` precisam de envelope para evoluir.
- Reaproveitar `domain.Item`: tem campos como `receipt_id` e `total_price` que não fazem sentido aqui; preço previous, average e purchase_count são exclusivos do search.

### Decisão 5 — Mobile: SearchBar inline e modo alternado

**Escolha:** `home_screen.dart` ganha:
1. `_SearchHeader` widget no topo do `body` (acima do `RefreshIndicator`), com `TextField` material e linha de `FilterChip`.
2. Estado novo: `_query`, `_periodDays` (int, default 30; `null` = "Tudo"), `_searchResults`, `_searchLoading`.
3. Debounce de 300ms via `Timer` instanciado em `_onQueryChanged`.
4. Quando `_query.trim().length < 3` → renderiza a lista de notas existente. Senão renderiza `_SearchResultsView` com `_searchResults`.
5. Tap no resultado → `Navigator.push(MaterialPageRoute(builder: (_) => ReceiptDetailScreen(apiClient: ..., receiptId: result.receiptId)))`.

Modelo `ItemSearchResult` em `lib/models/item_search_result.dart` espelha o DTO do backend; método `searchItems(query, periodDays)` em `services/api_client.dart`.

**Por quê:** Encaixa no padrão atual do app (StatefulWidget + setState) sem introduzir state library. Reusa `ReceiptDetailScreen` sem mudanças.

**Alternativas descartadas:**
- BottomNavigationBar com aba "Itens": mais escopo de UI; usuário escolheu SearchBar inline.
- Tela dedicada via IconButton: mais cliques pro mesmo resultado; menos descoberta.

### Decisão 6 — E2E

**Escolha:** Nova spec `testing/e2e/backend/tests/items/search.spec.ts`. Helpers em `state` (PreparedState) ganham `seedReceiptWithItems(houseId, store, items[], { issuedAt })` para criar receipts/items diretos no DB sem passar pelo worker (testes mais rápidos e determinísticos do que rodar scraping).

**Por quê:** Os cenários cobrem agregação, filtro temporal, isolation entre casas — coisas que só dependem da query SQL, não do scraper. Seed direto é mais barato e foca o teste no que importa.

## Risks / Trade-offs

- **[Risk] Permissão de `CREATE EXTENSION`** → ambiente local e a imagem `postgres:16-alpine` rodam como superuser por default. Em managed services (RDS) seria necessário pré-instalar; **mitigação:** documentar no README; a migration usa `IF NOT EXISTS` para ser idempotente.
- **[Risk] Descrições com whitespace/case diferentes não agrupam quando barcode é nulo** — itens "DETERGENTE YPE" e "DETERGENTE YPE  " contam como dois grupos. **Mitigação:** decisão consciente do usuário; podemos adicionar normalização (TRIM + UPPER) numa iteração futura se virar dor.
- **[Risk] Variação de preço induz ruído quando quantidade/unidade mudam** (ex: 1kg vs 500g) → `unit_price` da SEFAZ já é por unidade nominal (R$/kg ou R$/un), então geralmente é comparável; itens que mudam de unidade entre compras vão exibir variação grande. **Mitigação:** deixar como está; usuário pode interpretar.
- **[Risk] Performance do índice trigram em `unaccent(lower(description))`** com volume baixo é trivial, mas escrever um INSERT em `items` agora exige avaliar a função em uma expressão indexada. Custo de escrita marginal. **Mitigação:** aceitável.
- **[Trade-off] `truncated` sem total_count** — não dizemos "encontramos 73, mostrando 50". Para o uso pessoal isso é suficiente; pedir total seria mais um aggregate scan.

## Migration Plan

1. **Schema** — criar `007_item_search_indexes.up.sql` e `.down.sql`. Up: `CREATE EXTENSION` + dois índices. Down: `DROP INDEX` (não desinstala extensão para não quebrar futuros usos).
2. **Backend** — implementar query, handler, DTO, rota. Sem mudança em wire para clientes existentes.
3. **Mobile** — atualizar app; a versão antiga continua funcionando com o backend novo (não consome o endpoint).
4. **E2E** — adicionar spec; rodar suite local.
5. **Deploy** — `git push main`, CI builda imagem, VPS faz pull. A migration roda automaticamente no startup do `cmd/api`.

**Rollback:** `007_item_search_indexes.down.sql` derruba os índices; código que ainda os usasse passaria a fazer scan sequencial (não quebra). Para reverter completamente, revert do PR + redeploy.

## Open Questions

Nenhuma — pontas amarradas na rodada de perguntas. Itens explicitamente fora de escopo (tela de histórico, paginação, normalização de descrições) ficam registrados em Non-Goals para não voltarem como expectativa.
