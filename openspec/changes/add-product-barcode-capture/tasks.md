## 1. Migrations de banco de dados

- [x] 1.1 Criar `backend/migrations/005_item_barcode.up.sql` adicionando `barcode TEXT NULL` em `receipt_items`
- [x] 1.2 Criar `backend/migrations/005_item_barcode.down.sql` removendo a coluna
- [x] 1.3 Criar `backend/migrations/006_job_detail_phase.up.sql` adicionando `captcha_phase TEXT NULL` e `parsed_summary JSONB NULL` em `scraping_jobs`
- [x] 1.4 Criar `backend/migrations/006_job_detail_phase.down.sql` removendo essas colunas
- [x] 1.5 Subir o ambiente docker local e validar que as migrations aplicam sem erro

## 2. Modelo de domínio e queries (Go)

- [x] 2.1 Adicionar campo `Barcode *string` em `domain.ReceiptItem` (backend/internal/domain/models.go)
- [x] 2.2 Adicionar campo `CaptchaPhase *string` e `ParsedSummary *json.RawMessage` em `domain.ScrapingJob`
- [x] 2.3 Definir constantes `CaptchaPhaseSummary = "summary"` e `CaptchaPhaseDetail = "detail"` em `domain.models.go`
- [x] 2.4 Atualizar `CreateReceiptItem` em `database/queries.go` para incluir `barcode` no INSERT
- [x] 2.5 Atualizar `GetReceiptItems` / `GetReceiptByID` (ou equivalentes) para SELECT `barcode`
- [x] 2.6 Atualizar `FindActiveJobByURL`, `FindJobByReceiptID`, `GetByID` para SELECT `captcha_phase` e `parsed_summary`
- [x] 2.7 Atualizar `PauseJobForCaptcha` (ou handler equivalente) para aceitar `phase` e opcional `parsed_summary` como argumentos e persistir
- [x] 2.8 Atualizar `ResumeJobFromCaptcha` para manter `captcha_phase` e `parsed_summary` intactos (usar COALESCE no cookies/UA como já existe)

## 3. Parser da página detalhada

- [x] 3.1 Em `backend/internal/scraper/parser.go`, criar função `ExtractDetailLink(summaryHTML string) (string, error)` que encontra o href do botão "Ver NFC-e detalhada" e retorna a URL absoluta
- [x] 3.2 Criar função `ParseDetailPage(detailHTML string) ([]DetailItem, error)` que extrai a lista ordenada de EANs por item (retorna slice na mesma ordem que a página)
- [x] 3.3 Garantir que o parser trate `Código EAN Comercial` como fonte primária e `Código EAN Tributável` como fallback dentro do mesmo bloco de item
- [x] 3.4 Retornar EAN vazio (string vazia) para itens que não têm EAN cadastrado na detalhada — nunca falhar a função inteira por causa de um item
- [x] 3.5 Criar helper `MergeBarcodes(items []domain.ReceiptItem, barcodes []string) ([]domain.ReceiptItem, error)` que faz o merge por índice posicional e retorna erro se os tamanhos divergirem

## 4. Executor: navegação em duas fases

- [x] 4.1 Em `backend/internal/scraper/executor.go`, adicionar método `NavigateToDetail(ctx, detailURL)` que reaproveita o mesmo `browserCtx` e retorna `ExecutorResult` da página detalhada
- [x] 4.2 Alternativa simples se viável: expor `fetchPage` de forma que o worker possa encadear duas navegações na mesma sessão sem reinicializar o browser (refatorar `fetchPage` para retornar o `browserCtx` e um cleanup)
- [x] 4.3 Garantir que a detecção de captcha (`DetectCaptcha`) seja aplicada também ao resultado da navegação detalhada
- [x] 4.4 Na extração de cookies após pausa, capturar via `ExtractCurrentState` a URL da página detalhada (não a de challenge, se houver)

## 5. Worker: orquestração resumo → detalhe

- [x] 5.1 Em `backend/internal/scraper/worker.go`, após parse bem-sucedido da resumo, extrair detailURL via `ExtractDetailLink`
- [x] 5.2 Chamar `NavigateToDetail` na mesma sessão chromedp; detectar captcha no resultado
- [x] 5.3 Se captcha detectado na detalhada: persistir `parsed_summary` (serialização JSON do payload da resumo), `captcha_phase = 'detail'`, `captcha_current_url = detailURL`, cookies e UA; transicionar para `awaiting_captcha`; encerrar browser
- [x] 5.4 Se detalhada extraída com sucesso: chamar `ParseDetailPage`, `MergeBarcodes`, persistir receipt + items com barcode
- [x] 5.5 Se detalhada falhar por motivo não-captcha (timeout, parse): logar warning, persistir receipt + items sem barcode, marcar como `completed`
- [x] 5.6 Ajustar lógica de resume: ler `captcha_phase` do job; se `'detail'`, navegar direto para `captcha_current_url`, carregar `parsed_summary`, parsear só a detalhada, fazer merge e persistir
- [x] 5.7 Se `captcha_phase IS NULL` ou `'summary'`, manter comportamento atual (navegar em `fiscal_url`, parsear resumo, continuar pipeline)

## 6. API: campo barcode nas respostas

- [x] 6.1 Em `backend/internal/api/handlers.go` (ou struct de resposta correspondente), adicionar `Barcode *string `json:"barcode,omitempty"`` no DTO de item
- [x] 6.2 Popular o campo a partir de `domain.ReceiptItem.Barcode` nos handlers `GetReceipt` e `ListReceipts`
- [x] 6.3 Rodar `go vet ./...` e `gofmt -l .` — ambos devem sair limpos

## 7. Mobile: exibição do barcode

- [x] 7.1 Em `mobile/lib/models/receipt.dart`, adicionar campo `final String? barcode;` no modelo `ReceiptItem` e parseá-lo em `fromJson`
- [x] 7.2 Em `mobile/lib/screens/receipt_detail_screen.dart`, exibir o EAN abaixo ou ao lado do item quando `item.barcode != null && item.barcode!.isNotEmpty`
- [x] 7.3 Rodar `flutter analyze` — deve sair limpo
- [x] 7.4 Rodar `flutter test` — suítes existentes devem permanecer verdes

## 8. Testes E2E

- [x] 8.1 Adicionar fixture HTML da página detalhada em `testing/e2e/backend/mocks/sefaz/` (copiar estrutura real com 2-3 itens e EANs conhecidos)
- [x] 8.2 Estender `mock-server.js` para servir a rota `Nfe_DetalheCert.aspx` com essa fixture
- [x] 8.3 Novo spec `testing/e2e/backend/tests/receipts/barcode.spec.ts` cobrindo happy path: submeter receipt → aguardar `completed` → validar que items têm `barcode` correto no response
- [x] 8.4 Novo spec cobrindo captcha na fase detalhe: mock serve captcha na primeira requisição a `Nfe_DetalheCert.aspx`, job deve ir para `awaiting_captcha` com `captcha_phase = 'detail'`; resume com cookies mock deve completar o job com barcode
- [x] 8.5 Spec de regressão: garantir que o happy path existente (sem captcha) continua passando
- [x] 8.6 Rodar `npm test` completo — suíte deve estar verde

## 9. Validação manual

- [x] 9.1 Rebuild worker e API com `docker compose -f backend/tools/docker-compose.yml build`
- [x] 9.2 Restart dos containers
- [ ] 9.3 Flutter app instalado no dispositivo de teste (Samsung Galaxy S21 via ADB WiFi)
- [ ] 9.4 Escanear NFC-e real (SEFAZ SC); confirmar que os EANs aparecem no detalhe do recibo no app
- [ ] 9.5 Forçar cenário com captcha (nota diferente ou limpeza de cache) e confirmar que o fluxo WebView funciona em fase summary e em fase detail

## 10. Revisão e arquivamento

- [ ] 10.1 Rodar `openspec verify add-product-barcode-capture` para confirmar coerência
- [ ] 10.2 Commit com mensagem descritiva seguindo padrão Conventional Commits em PT-BR
- [ ] 10.3 Invocar `/opsx:archive` para arquivar o change após validação
