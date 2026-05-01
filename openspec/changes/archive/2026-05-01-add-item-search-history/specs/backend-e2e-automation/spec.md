## ADDED Requirements

### Requirement: Cobertura E2E para busca de itens

A suíte SHALL incluir cenários determinísticos para o endpoint `GET /api/v1/items/search`, exercitando agregação por barcode, agrupamento por descrição literal, filtro temporal, detecção de busca numérica, isolation por casa e estados degenerados.

#### Scenario: Itens com mesmo EAN são agregados em um único resultado
- **WHEN** a suíte popula a casa de teste com duas compras do mesmo `barcode` em descrições ligeiramente diferentes e busca pelo termo correspondente
- **THEN** a resposta retorna apenas um registro para esse EAN, com `purchase_count = 2` e `last_purchased_at` correspondente à compra mais recente

#### Scenario: Itens sem barcode permanecem separados por descrição
- **WHEN** a suíte popula duas compras com descrições distintas (sem `barcode`) que casam com a mesma query
- **THEN** a resposta retorna dois registros distintos

#### Scenario: Filtro temporal padrão exclui compras antigas
- **WHEN** a suíte popula uma compra com `issued_at` há 60 dias e outra com `issued_at` há 5 dias e busca sem `period_days`
- **THEN** apenas a compra dos últimos 30 dias aparece no resultado

#### Scenario: Busca numérica casa exatamente no barcode
- **WHEN** a suíte popula itens com diferentes códigos e busca com `q` igual a um EAN existente
- **THEN** apenas itens com aquele `barcode` exato são retornados

#### Scenario: Isolation entre casas é preservado
- **WHEN** a suíte popula um item correspondente em outra casa e o usuário autenticado pertence à casa A
- **THEN** o item da casa B não aparece nos resultados

#### Scenario: Query curta é rejeitada
- **WHEN** a suíte chama o endpoint com `q=ab`
- **THEN** o servidor responde `400 Bad Request`

#### Scenario: Variação de preço calculada corretamente
- **WHEN** a suíte popula duas compras do mesmo item com `unit_price` diferente e busca dentro do período
- **THEN** o resultado contém `last_unit_price` da compra mais recente e `previous_unit_price` da anterior
