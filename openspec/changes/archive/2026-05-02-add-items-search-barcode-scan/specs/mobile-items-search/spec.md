## ADDED Requirements

### Requirement: Botão de leitura de código de barras na barra de busca
O app mobile SHALL exibir um botão de leitura de código de barras (`Icons.barcode_reader`) no `suffixIcon` do campo de busca de itens da `HomeScreen`. O botão SHALL abrir a tela de leitura de código de barras de produto ao ser tocado, e SHALL ficar oculto sempre que o campo de busca contiver texto não-vazio (nessa situação, apenas o botão de limpar `Icons.close` permanece visível).

#### Scenario: Campo de busca vazio mostra botão da câmera
- **WHEN** o campo de busca da `HomeScreen` está vazio
- **THEN** o `suffixIcon` exibe um único `IconButton` com o ícone `Icons.barcode_reader` que, ao ser tocado, navega para a tela de leitura de código de barras de produto

#### Scenario: Campo de busca com texto oculta o botão da câmera
- **WHEN** o usuário digitou ao menos um caractere no campo de busca
- **THEN** o `suffixIcon` exibe apenas o `IconButton` de limpar (`Icons.close`); o botão da câmera não aparece

#### Scenario: Toque no botão com permissão de câmera negada permanentemente
- **WHEN** o usuário toca o botão de leitura mas a permissão de câmera foi negada permanentemente em sessões anteriores
- **THEN** a tela do scanner abre e exibe a UI de "permissão negada" com link para `openAppSettings`, sem travar nem abrir uma câmera vazia

### Requirement: Tela de leitura de código de barras de produto
O app SHALL fornecer uma tela `ProductBarcodeScannerScreen` que usa o pacote `mobile_scanner` configurado para os formatos 1D mais comuns em produtos brasileiros (EAN-13, EAN-8, UPC-A, UPC-E, Code-128, ITF) e devolve à `HomeScreen` apenas resultados que contenham exclusivamente dígitos com 8 ou mais caracteres.

#### Scenario: Detecção de EAN válido devolve o número à busca
- **WHEN** a tela detecta um código de barras cujo `rawValue` casa a expressão regular `^\d{8,}$`
- **THEN** o app dispara `HapticFeedback.lightImpact()`, fecha a tela do scanner e retorna a string detectada para a `HomeScreen`, que SHALL preencher o `TextEditingController` da busca com esse valor e SHALL disparar o fluxo de busca existente (mesmo caminho de uma digitação manual)

#### Scenario: Detecção fora do padrão é rejeitada e câmera continua escaneando
- **WHEN** a tela detecta um código cujo `rawValue` contém caracteres não numéricos ou tem menos de 8 dígitos
- **THEN** o app exibe um `SnackBar` com mensagem indicando que o código não é um EAN reconhecido, NÃO dispara haptic, NÃO fecha a tela, e continua escaneando

#### Scenario: Formatos 2D não são detectados nesta tela
- **WHEN** o usuário aponta a câmera para um QR code (incluindo um QR de NFC-e válido)
- **THEN** o detector da `ProductBarcodeScannerScreen` ignora o código (formato `BarcodeFormat.qrCode` não está habilitado), evitando ambiguidade com a tela de leitura de NFC-e

#### Scenario: Cancelamento sem detecção
- **WHEN** o usuário fecha a tela do scanner pelo botão de voltar do `AppBar` antes de qualquer detecção válida
- **THEN** o controller de busca da `HomeScreen` permanece com o valor que tinha antes da abertura da tela; nenhum estado de busca é alterado

### Requirement: Controles secundários na tela do scanner de produto
A `ProductBarcodeScannerScreen` SHALL oferecer um botão de lanterna no `AppBar` que liga e desliga o flash da câmera, e SHALL renderizar um overlay visual de "mira" sobreposto à pré-visualização da câmera.

#### Scenario: Lanterna inicia apagada e alterna ao toque
- **WHEN** a tela do scanner é aberta
- **THEN** a lanterna está apagada por padrão; o `AppBar` exibe um `IconButton` com `Icons.flash_off` que, ao ser tocado, chama `MobileScannerController.toggleTorch()` e atualiza o ícone para `Icons.flash_on` (e vice-versa) reativamente

#### Scenario: Lanterna não persiste entre aberturas
- **WHEN** o usuário liga a lanterna, fecha a tela e a abre novamente
- **THEN** a lanterna está apagada na nova abertura (estado não é persistido)

#### Scenario: Overlay de mira é exibido sobre a câmera
- **WHEN** a tela do scanner está ativa com permissão de câmera concedida
- **THEN** sobre a pré-visualização da câmera é renderizado um overlay com máscara escurecida e uma janela retangular transparente de proporção wide (~3.5:1) centralizada, indicando ao usuário onde enquadrar o código

### Requirement: Reaproveitamento da regra de busca por EAN do backend
O app SHALL preencher o campo de busca com o EAN escaneado de forma que o fluxo subsequente seja idêntico ao de uma digitação manual de 8+ dígitos, sem chamar nenhum endpoint adicional. A interpretação como busca exata por `barcode` SHALL ser feita exclusivamente pelo backend (capability `item-search-history`).

#### Scenario: Mesmo endpoint, mesmo debounce
- **WHEN** o EAN é retornado para a `HomeScreen`
- **THEN** o app delega a `_onQueryChanged(ean)`, que aplica o mesmo debounce de 300ms e chama `ApiClient.searchItems(ean, periodDays: <atual>)` — sem rota nova, sem flag adicional na requisição
