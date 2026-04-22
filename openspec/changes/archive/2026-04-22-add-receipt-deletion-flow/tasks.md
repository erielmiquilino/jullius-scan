## 1. Backend - receipt deletion API

- [x] 1.1 Criar a migração `003_receipt_deletion_fk.up.sql` para recriar a foreign key de `scraping_jobs.receipt_id` com `ON DELETE SET NULL`
- [x] 1.2 Criar a migração `003_receipt_deletion_fk.down.sql` revertendo a foreign key para o comportamento anterior
- [x] 1.3 Adicionar em `internal/database/queries.go` um método transacional para excluir recibo por `id` e `house_id`, limpando referências em `scraping_jobs.receipt_id` antes ou durante a remoção
- [x] 1.4 Implementar `DELETE /api/v1/receipts/{id}` em `internal/api/handlers.go` com parse do parâmetro, validação house-scoped e respostas consistentes para sucesso, `404` e `403`
- [x] 1.5 Registrar a rota `DELETE /api/v1/receipts/{id}` em `internal/api/router.go`

## 2. Mobile - destructive flow on receipt detail

- [x] 2.1 Adicionar `deleteReceipt(int id)` em `mobile/lib/services/api_client.dart`
- [x] 2.2 Atualizar `mobile/lib/screens/receipt_detail_screen.dart` para exibir a ação de remover o lançamento na AppBar ou em CTA equivalente
- [x] 2.3 Implementar `AlertDialog` de confirmação com pergunta explícita antes da deleção e opção de cancelar
- [x] 2.4 Bloquear toques repetidos durante a deleção, tratar erro de API e manter a tela aberta se a remoção falhar
- [x] 2.5 Em caso de sucesso, retornar da detail screen com resultado positivo para sinalizar refresh da listagem

## 3. Mobile - list refresh after deletion

- [x] 3.1 Atualizar `mobile/lib/screens/home_screen.dart` para aguardar o retorno da `ReceiptDetailScreen`
- [x] 3.2 Quando a detail screen retornar sucesso de exclusão, chamar `_loadReceipts()` antes de exibir novamente a lista
- [x] 3.3 Garantir que o item removido não apareça mais após a navegação de volta, inclusive no estado de pull-to-refresh subsequente

## 4. Validation

- [x] 4.1 Adicionar ou atualizar testes E2E em `testing/e2e/backend/` cobrindo deleção bem-sucedida, `404` e isolamento por `House`
- [x] 4.2 Adicionar testes Flutter para confirmação, cancelamento, erro de deleção e retorno com refresh da lista
- [x] 4.3 Rodar `cd backend && go vet ./... && gofmt -l .`
- [x] 4.4 Rodar `cd testing/e2e/backend && npm test`
- [x] 4.5 Rodar `cd mobile && flutter analyze && flutter test`
