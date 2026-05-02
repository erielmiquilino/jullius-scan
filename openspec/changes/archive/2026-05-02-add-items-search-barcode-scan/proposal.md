## Why

A busca de itens no app mobile já reconhece automaticamente quando a query é um EAN (8+ dígitos) e faz busca exata por `barcode`. Hoje, porém, o usuário precisa digitar manualmente os 13 dígitos do código de barras de um produto físico — o que é tedioso, propenso a erro e desperdiça uma capacidade que o aparelho já tem (a câmera, já usada para escanear QR codes de NFC-e). Permitir que o usuário aponte a câmera para um EAN no produto e tenha a query preenchida automaticamente fecha o ciclo da proposta de valor "consultar histórico de preços de um item específico" sem digitação.

## What Changes

- Adicionar um botão à direita do campo de busca em `HomeScreen` (`Icons.barcode_reader`) que abre uma nova tela de leitura de código de barras de produto. O botão SHALL ficar oculto quando o campo já contiver texto (apenas o botão "X" de limpar permanece visível) para evitar conflito visual.
- Implementar a tela `ProductBarcodeScannerScreen` reaproveitando o pacote `mobile_scanner` (já dependência do app), porém configurada para os formatos 1D usados em rótulos brasileiros (EAN-13, EAN-8, UPC-A, UPC-E, Code-128, ITF). É distinta da tela `QrScannerScreen` que aceita apenas QR de NFC-e — mas as duas SHALL compartilhar widgets comuns (overlay de mira, botão de lanterna, gate de permissão de câmera) extraídos para `mobile/lib/features/scanner/widgets/`.
- A validação client-side aceita o resultado quando `rawValue` for composto exclusivamente por dígitos com 8+ caracteres (espelha exatamente a regra do backend: `q` numérico de 8+ dígitos ⇒ busca exata por `barcode`); detecções fora desse padrão geram SnackBar e a câmera continua escaneando.
- Ao detectar um código válido, a tela retorna a string numérica para `HomeScreen`, que preenche o `TextEditingController` da busca e dispara o fluxo existente de busca; nenhuma chamada nova de backend é necessária.
- **Aplicar os mesmos polimentos à `QrScannerScreen` existente** (escopo ampliado por solicitação do usuário): overlay com retângulo de mira (proporção quadrada para QR; retangular wide para o scanner de produto), feedback háptico `HapticFeedback.lightImpact()` quando uma detecção válida ocorre, e botão de lanterna (`toggleTorch`) no `AppBar`. Reuso de código via widgets compartilhados.
- Tratar permissão de câmera com a mesma UX já existente (permission handler, tela de "permissão negada" com link para configurações); o fluxo de permissão também é extraído para o widget compartilhado.

## Capabilities

### New Capabilities
- `mobile-items-search`: UX de busca de itens no app mobile — campo de busca, chips de período, e agora a entrada por leitura de código de barras. Cobre apenas a camada de apresentação do app Flutter; o contrato HTTP do endpoint permanece em `item-search-history`.

### Modified Capabilities
- `mobile-qrcode-capture`: a tela de leitura de QR de NFC-e ganha overlay de mira, feedback háptico em detecções válidas e botão de lanterna — UX que passa a ser obrigatória, não opcional. O contrato funcional (formatos suportados, validação por regex `_nfceUrlPattern`, fluxo de permissão) permanece igual; muda o que o usuário vê e sente.

## Impact

- **Mobile (Flutter):**
  - `mobile/lib/screens/home_screen.dart`: novo `IconButton` no `suffixIcon` da `TextField` (oculto quando há texto), handler que navega para o scanner e preenche o controller com o EAN retornado.
  - `mobile/lib/features/scanner/product_barcode_scanner_screen.dart` (novo): tela com `MobileScanner` configurada para formatos 1D + validação client-side (regex `^\d{8,}$`).
  - `mobile/lib/features/scanner/qr_scanner_screen.dart`: refatorada para consumir os mesmos widgets compartilhados (mira, torch, permission gate) e disparar haptic em detecção válida.
  - `mobile/lib/features/scanner/widgets/scanner_viewfinder.dart` (novo): overlay com máscara escurecida e janela retangular de mira; aspect ratio parametrizável (1:1 para QR, ~4:1 para barcode 1D).
  - `mobile/lib/features/scanner/widgets/camera_permission_gate.dart` (novo): widget que encapsula a lógica `Permission.camera.request()` + tela de "permissão negada" com link para `openAppSettings`.
  - `mobile/lib/features/scanner/widgets/torch_button.dart` (novo, opcional — pode virar helper inline): `IconButton` plugado em `MobileScannerController.toggleTorch()` com ícone reativo ao estado da lanterna.
  - Reaproveitamento de `mobile_scanner` e `permission_handler` — nenhuma nova dependência em `pubspec.yaml`.
- **Backend / E2E:** sem alterações. O endpoint `GET /api/v1/items/search` já trata `q` numérico de 8+ dígitos como busca exata por EAN.
- **Permissões nativas:** `android/app/src/main/AndroidManifest.xml` e `ios/Runner/Info.plist` já declaram `CAMERA` / `NSCameraUsageDescription` para o scanner de QR — nenhuma mudança necessária.
- **Testes:** sem cobertura E2E automatizada (mobile E2E está reservado mas não habilitado, conforme `CLAUDE.md`); validação manual em emulador/aparelho físico.
