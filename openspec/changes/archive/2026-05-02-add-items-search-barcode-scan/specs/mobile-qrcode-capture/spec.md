## ADDED Requirements

### Requirement: Feedback háptico em detecção válida de QR de NFC-e
A tela de leitura de QR de NFC-e SHALL disparar `HapticFeedback.lightImpact()` exatamente uma vez no instante em que uma detecção é considerada válida (URL casa o padrão `_nfceUrlPattern`), antes de fechar a tela e retornar a URL normalizada para o chamador.

#### Scenario: QR válido vibra antes de fechar
- **WHEN** a câmera detecta um QR code cuja URL casa `_nfceUrlPattern`
- **THEN** o app dispara `HapticFeedback.lightImpact()` e em seguida chama `Navigator.pop` com a URL normalizada

#### Scenario: QR inválido não vibra
- **WHEN** a câmera detecta um QR code cuja URL NÃO casa `_nfceUrlPattern`
- **THEN** o app exibe a mensagem de erro existente, NÃO dispara haptic e NÃO fecha a tela

### Requirement: Botão de lanterna no scanner de NFC-e
A tela de leitura de QR de NFC-e SHALL exibir, no `AppBar`, um `IconButton` que alterna o flash da câmera via `MobileScannerController.toggleTorch()`. O ícone SHALL refletir o estado atual da lanterna de forma reativa.

#### Scenario: Estado inicial e alternância
- **WHEN** a tela é aberta e o usuário toca o botão de lanterna
- **THEN** o estado inicial da lanterna é desligado (ícone `Icons.flash_off`); ao tocar, a lanterna acende e o ícone passa a `Icons.flash_on`; ao tocar novamente, apaga e volta a `Icons.flash_off`

#### Scenario: Lanterna não persiste entre aberturas
- **WHEN** o usuário liga a lanterna, fecha a tela e a reabre
- **THEN** a lanterna está apagada na nova abertura

### Requirement: Overlay de mira no scanner de NFC-e
A tela de leitura de QR de NFC-e SHALL renderizar, sobre a pré-visualização da câmera, um overlay com máscara escurecida e uma janela transparente de proporção quadrada (1:1) centralizada, indicando ao usuário onde enquadrar o QR code. O overlay SHALL ser puramente visual e NÃO SHALL restringir a área de detecção real do `mobile_scanner`.

#### Scenario: Mira aparece quando a câmera está ativa
- **WHEN** a tela do scanner está ativa com permissão de câmera concedida
- **THEN** o overlay com janela quadrada centralizada é renderizado por cima da pré-visualização da câmera

#### Scenario: Detecção fora do alvo visual ainda funciona
- **WHEN** o usuário aponta um QR code válido em uma região fora da janela transparente do overlay
- **THEN** o detector ainda reconhece e processa o código (o overlay é apenas visual; não há restrição de `scanWindow`)

## MODIFIED Requirements

### Requirement: Camera permission handling
The mobile app SHALL request and gracefully handle the camera permission required by the QR scanner. The permission flow SHALL be implemented through a shared widget (`CameraPermissionGate`) reused by all camera-based scanner screens in the app, ensuring consistent behavior across the QR-de-NFC-e scanner and any other scanner screen.

#### Scenario: Camera permission is denied
- **WHEN** the user denies the camera permission request on the scanner screen
- **THEN** the app displays an explanation with an action to open system settings, and does not crash or leave the scanner in a frozen state

#### Scenario: Camera permission is granted after previous denial
- **WHEN** the user grants the camera permission after initially denying it
- **THEN** the scanner becomes functional without requiring the user to restart the app

#### Scenario: Permission UI consistency across scanner screens
- **WHEN** the user encounters the "permission denied" state in any scanner screen of the app
- **THEN** the visual layout, copy and call-to-action button (linking to `openAppSettings`) are identical, because all scanner screens consume the same `CameraPermissionGate` widget
