# Spec: Backend E2E Automation

## Purpose

Defines behavioural requirements for the backend end-to-end test automation platform — the isolated Playwright/TypeScript workspace that validates the full backend flow across API, worker, PostgreSQL, and Redis.

---
## Requirements
### Requirement: Separate backend E2E workspace
The system SHALL provide a backend E2E automation workspace that is physically separated from the main backend runtime code and from any future mobile E2E suite.

#### Scenario: Keep backend runtime free from E2E Node dependencies
- **WHEN** the backend E2E suite is installed and executed
- **THEN** its Playwright and TypeScript dependencies live in a dedicated automation workspace rather than inside the main Go backend module

#### Scenario: Preserve future mobile E2E separation
- **WHEN** the repository later adds a Flutter mobile E2E suite
- **THEN** that suite can be created in a parallel workspace without sharing the same root commands, config files, or suite directory with backend E2E

### Requirement: End-to-end backend flow coverage
The backend E2E suite SHALL validate the real application flow across HTTP API, asynchronous worker processing, Redis queueing, and PostgreSQL persistence.

#### Scenario: Validate successful receipt processing flow
- **WHEN** the suite submits a valid receipt extraction request through the API in the test environment
- **THEN** it verifies that a job is created, processed asynchronously by the worker, and persisted as normalized receipt data in PostgreSQL

#### Scenario: Validate final result retrieval
- **WHEN** a test receipt processing flow completes successfully
- **THEN** the suite verifies that the API exposes the completed job and receipt result expected for the requesting House context

### Requirement: Deterministic test environment orchestration
The backend E2E suite SHALL run against a reproducible environment that includes isolated API, worker, PostgreSQL, and Redis services suitable for local and CI execution.

#### Scenario: Start isolated backend E2E stack
- **WHEN** a developer or CI job starts the backend E2E suite
- **THEN** the required services run in an isolated test environment that does not depend on shared long-lived development infrastructure

#### Scenario: Avoid mandatory external SEFAZ dependency
- **WHEN** the backend E2E suite runs its deterministic scenarios
- **THEN** it does not require live public SEFAZ availability to validate the internal backend flow

#### Scenario: Serve SEFAZ fixture from local mock server
- **WHEN** the deterministic backend E2E stack starts
- **THEN** it includes a simple local mock server that exposes the static fixture `nfce-consulta-detalhada.html` from the backend E2E workspace for the Go worker to fetch during tests

### Requirement: Real Firebase authentication in E2E
The backend E2E suite SHALL authenticate through a real dedicated Firebase project using the Firebase REST API, without adding any authentication bypass logic to the Go product code.

#### Scenario: Obtain real JWT for API requests
- **WHEN** a backend E2E scenario needs to call an authenticated API endpoint
- **THEN** the suite signs in through the Firebase REST API using environment-provided test credentials and reuses the resulting JWT against the Go API

#### Scenario: Preserve product authentication path
- **WHEN** backend E2E automation is added to the repository
- **THEN** the Go API continues to validate Firebase tokens through its normal product path rather than through test-only bypass behavior

### Requirement: Controlled seed and reset workflow
The backend E2E suite SHALL initialize PostgreSQL and Redis into a known state before tests through explicit seed and cleanup mechanisms.

#### Scenario: Reset state before execution
- **WHEN** a backend E2E test run starts
- **THEN** the suite clears or recreates relevant database and Redis state so prior runs do not influence results

#### Scenario: Seed required business context
- **WHEN** a scenario needs authenticated users, Houses, memberships, jobs, or receipts
- **THEN** the suite provisions only the explicit test fixtures required for that scenario in a repeatable way

#### Scenario: Reset and inspect state from TypeScript utilities
- **WHEN** the suite prepares or verifies test state
- **THEN** it can connect directly to PostgreSQL and Redis from the backend E2E workspace using dedicated TypeScript clients to seed, clean, inspect, and preload deterministic conditions

### Requirement: Async-aware verification helpers
The backend E2E suite SHALL provide centralized waiting helpers for asynchronous job completion and failure diagnosis.

#### Scenario: Poll until job reaches terminal state
- **WHEN** a scenario triggers worker processing that completes asynchronously
- **THEN** the suite waits using bounded polling helpers instead of relying on arbitrary fixed sleeps as the primary synchronization strategy

#### Scenario: Emit diagnosable timeout failures
- **WHEN** a job does not reach the expected state within the configured E2E timeout
- **THEN** the suite fails with diagnostics that identify the last observed API or persistence state

### Requirement: Suite-scoped commands and artifacts
The backend E2E suite SHALL expose its own execution commands, reports, and failure artifacts independently from future mobile E2E automation.

#### Scenario: Run backend E2E alone
- **WHEN** a developer runs the backend E2E command set
- **THEN** only the backend automation workspace configuration, tests, and reports are involved

#### Scenario: Collect failure artifacts for CI
- **WHEN** a backend E2E scenario fails in local execution or CI
- **THEN** the suite stores runner artifacts such as logs, traces, or reports in a backend-E2E-specific location

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

