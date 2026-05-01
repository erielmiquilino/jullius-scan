# item-search-history Specification

## Purpose
TBD - created by archiving change add-item-search-history. Update Purpose after archive.
## Requirements
### Requirement: Endpoint de busca agregada de itens

O sistema SHALL expor um endpoint autenticado `GET /api/v1/items/search` que retorna itens já comprados pela casa do usuário, agrupados, com resumo da última compra e estatísticas do período. A resposta SHALL ser limitada a 50 resultados ordenados por última compra (descendente).

#### Scenario: Resposta inclui resumo de última compra e estatísticas do período
- **WHEN** um usuário autenticado chama `GET /api/v1/items/search?q=banana` e existem itens correspondentes na sua casa
- **THEN** a resposta retorna até 50 objetos contendo `description`, `barcode` (quando houver), `last_purchased_at` (RFC3339), `last_unit_price`, `last_total_price`, `previous_unit_price` (null se houver apenas uma compra), `average_unit_price` (ponderado por quantidade), `purchase_count`, `store` (id, name, cnpj) e `receipt_id` da última compra

#### Scenario: Ordenação dos resultados
- **WHEN** o endpoint retorna múltiplos resultados
- **THEN** os resultados são ordenados por `last_purchased_at` em ordem descendente

#### Scenario: Limite de resultados
- **WHEN** mais de 50 itens correspondem à busca dentro do período
- **THEN** a resposta retorna no máximo 50 itens (os mais recentes pela última compra) e inclui um indicador `truncated: true` no envelope

### Requirement: Agrupamento por barcode quando disponível

O sistema SHALL agregar todas as linhas de itens que compartilham o mesmo `barcode` em um único resultado, independentemente de variações na descrição livre. Itens sem `barcode` SHALL ser agrupados pela descrição literal exata.

#### Scenario: Itens com mesmo EAN colapsam em uma linha
- **WHEN** a casa tem múltiplas compras de itens com `barcode = '7891234567890'` mas descrições variadas (ex: "BANANA NANICA KG", "BANANA NANICA")
- **THEN** o resultado da busca exibe um único registro para esse EAN, usando a descrição da última compra como representativa

#### Scenario: Itens sem barcode permanecem separados por descrição literal
- **WHEN** a casa tem itens "DETERGENTE YPE 500ML" e "DETERGENTE YPE" sem `barcode`
- **THEN** o resultado da busca exibe dois registros distintos, um para cada descrição literal

### Requirement: Filtro temporal padrão e configurável

O sistema SHALL aplicar um filtro padrão de últimos 30 dias com base em `receipts.issued_at`. Os clientes SHALL poder ampliar o período via query string ou desabilitá-lo para considerar todo o histórico.

#### Scenario: Filtro padrão de 30 dias
- **WHEN** a busca é feita sem o parâmetro `period_days`
- **THEN** apenas itens cuja `receipts.issued_at` está nos últimos 30 dias são considerados

#### Scenario: Período ampliado
- **WHEN** a busca é feita com `period_days=180`
- **THEN** apenas itens cuja `receipts.issued_at` está nos últimos 180 dias são considerados

#### Scenario: Histórico completo
- **WHEN** a busca é feita com `period_days=null` (parâmetro com valor explícito `null` ou string vazia)
- **THEN** nenhum filtro temporal é aplicado

#### Scenario: Período inválido
- **WHEN** a busca é feita com `period_days` não numérico ou negativo
- **THEN** o servidor retorna `400 Bad Request` com mensagem identificando o parâmetro inválido

### Requirement: Detecção automática de busca por barcode vs descrição

O sistema SHALL interpretar a query como busca exata em `barcode` quando seu valor consistir apenas em 8 ou mais dígitos contínuos. Caso contrário, SHALL aplicar busca por substring case-insensitive e tolerante a acentos sobre `description`.

#### Scenario: Busca exata por código de barras
- **WHEN** a query é `q=7891234567890`
- **THEN** o resultado retorna apenas itens com `barcode = '7891234567890'`

#### Scenario: Busca por descrição com tolerância a acentos
- **WHEN** a query é `q=acai` e existem itens com descrição "AÇAÍ POLPA 500G"
- **THEN** os resultados incluem esses itens (busca case-insensitive e diacrítico-insensitive)

#### Scenario: Query mista alfanumérica é tratada como descrição
- **WHEN** a query é `q=COCA 2L`
- **THEN** o sistema aplica busca por substring em `description`, não por barcode

### Requirement: Tamanho mínimo de query

O sistema SHALL exigir que `q` contenha pelo menos 3 caracteres não-brancos. Buscas mais curtas SHALL ser rejeitadas com `400 Bad Request`.

#### Scenario: Query curta rejeitada
- **WHEN** a busca é feita com `q=ab`
- **THEN** o servidor retorna `400 Bad Request` com mensagem identificando o requisito de tamanho mínimo

#### Scenario: Espaços não contam para o mínimo
- **WHEN** a busca é feita com `q=  ab  `
- **THEN** o servidor retorna `400 Bad Request`

### Requirement: Escopo por casa do usuário autenticado

O sistema SHALL restringir os resultados aos itens pertencentes à casa do usuário autenticado, derivada do middleware de resolução de casa. Itens de outras casas NÃO SHALL ser retornados em hipótese alguma.

#### Scenario: Isolation entre casas
- **WHEN** um usuário da casa A faz uma busca por um termo presente apenas em itens da casa B
- **THEN** o resultado é uma lista vazia

#### Scenario: Requisição não autenticada
- **WHEN** a busca é feita sem token Firebase válido
- **THEN** o servidor retorna `401 Unauthorized`

### Requirement: Variação e estatísticas de preço

O sistema SHALL calcular `average_unit_price` como média ponderada de `unit_price` por `quantity` dentro do período filtrado, e `previous_unit_price` como o `unit_price` da penúltima compra do mesmo grupo dentro do período.

#### Scenario: Média ponderada por quantidade
- **WHEN** existem duas compras do item: 2kg a R$ 4,00 e 1kg a R$ 7,00
- **THEN** `average_unit_price` retorna 5.00 (= (2*4 + 1*7) / 3)

#### Scenario: Penúltima compra disponível
- **WHEN** existem múltiplas compras do mesmo item dentro do período
- **THEN** `previous_unit_price` é o `unit_price` da segunda compra mais recente

#### Scenario: Apenas uma compra no período
- **WHEN** existe apenas uma compra do item dentro do período
- **THEN** `previous_unit_price` é `null` (cliente exibe variação como neutra)

### Requirement: Indexes e extensões de banco

O sistema SHALL provisionar via migration as extensões `pg_trgm` e `unaccent` no banco e criar índices que suportem busca trigram em descrição e lookup exato em barcode.

#### Scenario: Extensões habilitadas
- **WHEN** a migration de busca de itens é aplicada em um banco que ainda não as possui
- **THEN** `CREATE EXTENSION IF NOT EXISTS pg_trgm` e `CREATE EXTENSION IF NOT EXISTS unaccent` são executados sem erro

#### Scenario: Índice trigram para descrição
- **WHEN** a migration é aplicada
- **THEN** existe um índice GIN sobre `unaccent(lower(description))` na tabela `items` que cobre o operador `gin_trgm_ops`

#### Scenario: Índice de barcode
- **WHEN** a migration é aplicada
- **THEN** existe um índice B-tree sobre `items.barcode` (parcial em `barcode IS NOT NULL`) que suporta lookup exato

### Requirement: Busca de itens no mobile via SearchBar inline

O app mobile SHALL exibir uma SearchBar e uma linha de chips de período acima da lista do `home_screen`. Quando a query tiver pelo menos 3 caracteres, a lista de notas SHALL ser substituída pela lista de itens, com debounce de 300ms entre digitação e chamada à API.

#### Scenario: SearchBar vazio mantém comportamento atual
- **WHEN** a SearchBar está vazia
- **THEN** o conteúdo do `home_screen` é a lista de recibos atual

#### Scenario: Digitação dispara busca após 3 caracteres
- **WHEN** o usuário digita "ban" e aguarda 300ms sem novas teclas
- **THEN** o app chama o endpoint de busca de itens uma única vez e exibe os resultados

#### Scenario: Chips selecionam o período
- **WHEN** o usuário toca em um chip diferente do atual com a SearchBar preenchida
- **THEN** o app refaz a busca com o novo `period_days` e atualiza a lista exibida

#### Scenario: Tap em resultado abre a nota da última compra
- **WHEN** o usuário toca em um item dos resultados de busca
- **THEN** o app navega para `ReceiptDetailScreen` carregando o `receipt_id` da última compra retornado pela API

### Requirement: Card de resultado de busca

Cada card de resultado SHALL exibir descrição, EAN (quando houver), data da última compra, último preço unitário formatado em R$, nome da loja, contador de compras no período, preço médio do período e indicador visual de variação de preço comparando com a compra imediatamente anterior.

#### Scenario: Variação para baixo
- **WHEN** o `last_unit_price` é menor que o `previous_unit_price`
- **THEN** o card exibe um indicador verde com seta para baixo e o delta absoluto em R$

#### Scenario: Variação para cima
- **WHEN** o `last_unit_price` é maior que o `previous_unit_price`
- **THEN** o card exibe um indicador vermelho com seta para cima e o delta absoluto em R$

#### Scenario: Sem compra anterior
- **WHEN** `previous_unit_price` é nulo
- **THEN** o card omite o indicador de variação ou o exibe em estado neutro (cinza, sem seta)

#### Scenario: Item sem EAN
- **WHEN** `barcode` é nulo no resultado
- **THEN** o card omite a linha de EAN

