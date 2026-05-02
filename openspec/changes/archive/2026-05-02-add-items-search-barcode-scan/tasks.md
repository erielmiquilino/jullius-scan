## 1. Widgets compartilhados de scanner

- [x] 1.1 Criar `mobile/lib/features/scanner/widgets/camera_permission_gate.dart` — `StatefulWidget` que chama `Permission.camera.request()` no `initState` e renderiza `child` quando concedida ou a tela "permissão negada" com `openAppSettings` quando não. Contrato: `CameraPermissionGate({required Widget child})`.
- [x] 1.2 Criar `mobile/lib/features/scanner/widgets/scanner_viewfinder.dart` — `StatelessWidget` overlay com máscara `Color.black54` e janela retangular transparente centralizada de cantos arredondados. Contrato: `ScannerViewfinder({required double aspectRatio, double widthFraction = 0.8})`.
- [x] 1.3 Validar manualmente os widgets compartilhados num app de teste mínimo ou via `flutter run` na `QrScannerScreen` antes de migrar a tela inteira (sanity check de layout/desempenho). _Validado: build debug instalado no Galaxy S21, layout dos widgets compartilhados aprovado pelo usuário._

## 2. Refatoração da `QrScannerScreen` para os novos widgets e UX

- [x] 2.1 Substituir a lógica inline de permissão de câmera (`_checkCameraPermission`, `_cameraPermissionDenied`, `_buildPermissionDenied`) pelo wrap `CameraPermissionGate(child: _buildScanner())`.
- [x] 2.2 Empilhar `ScannerViewfinder(aspectRatio: 1.0)` sobre o `MobileScanner` em `_buildScanner`.
- [x] 2.3 Adicionar `IconButton` de lanterna no `actions` do `AppBar`, plugado em `MobileScannerController.toggleTorch()` via `ValueListenableBuilder<TorchState>`. Manter o `IconButton` de teclado existente para fallback manual.
- [x] 2.4 Em `_onDetect`, disparar `HapticFeedback.lightImpact()` no caminho de sucesso (URL casa `_nfceUrlPattern`), antes de `Navigator.pop`. Não vibrar no caminho de rejeição.
- [x] 2.5 Rodar `flutter analyze` e validar manualmente no emulador Android: scan de QR válido (vibra, fecha), QR inválido (mensagem, sem vibração), lanterna alterna, mira aparece, "permissão negada" segue exibindo o link para settings. _`flutter analyze` limpo; validação manual no Galaxy S21 aprovada pelo usuário._

## 3. Tela de leitura de código de barras de produto

- [x] 3.1 Criar `mobile/lib/features/scanner/product_barcode_scanner_screen.dart` com `MobileScanner` configurado para `formats: [BarcodeFormat.ean13, BarcodeFormat.ean8, BarcodeFormat.upcA, BarcodeFormat.upcE, BarcodeFormat.code128, BarcodeFormat.itf]` e wrap em `CameraPermissionGate`.
- [x] 3.2 Implementar `_onDetect` que valida `rawValue` contra `RegExp(r'^\d{8,}$')`. Em caso de sucesso: `HapticFeedback.lightImpact()` + `Navigator.pop(context, rawValue)`. Em caso de falha: `SnackBar` "Código não reconhecido como EAN" e continuar escaneando (não fecha a tela).
- [x] 3.3 Empilhar `ScannerViewfinder(aspectRatio: 3.5)` sobre o `MobileScanner`.
- [x] 3.4 Adicionar `IconButton` de lanterna no `AppBar` com o mesmo padrão da tela de QR.
- [x] 3.5 Estado inicial: lanterna apagada; tela aberta sem persistência de estado entre aberturas.

## 4. Integração na barra de busca da `HomeScreen`

- [x] 4.1 Em `mobile/lib/screens/home_screen.dart`, alterar o `suffixIcon` do `TextField` em `_SearchHeader` para retornar `IconButton(icon: Icons.barcode_reader, onPressed: onScanBarcode)` quando `controller.text.isEmpty`, e manter o `IconButton(icon: Icons.close)` quando há texto. Adicionar `onScanBarcode: VoidCallback` ao construtor de `_SearchHeader` e propagar do `_HomeScreenState`.
- [x] 4.2 Implementar `Future<void> _onScanBarcode()` em `_HomeScreenState` que faz `Navigator.push<String>(... ProductBarcodeScannerScreen)`, e ao receber `ean != null`: `_searchController.text = ean` e `_onQueryChanged(ean)` (reaproveita debounce + `_runSearch`).
- [x] 4.3 Garantir que o botão da câmera não aparece simultaneamente ao botão de limpar (cobre o cenário "Campo de busca com texto oculta o botão da câmera" do spec).
- [x] 4.4 Importar `ProductBarcodeScannerScreen` em `home_screen.dart` (manter `qr_scanner_screen.dart` import existente intacto).

## 5. Validação e gates

- [x] 5.1 Rodar `flutter analyze` na raiz do `mobile/` — output deve ficar limpo (sem warnings introduzidos pela mudança). _Resultado: `No issues found!`._
- [x] 5.2 Rodar `flutter test` (mesmo que os testes existentes sejam mínimos, garantir que nada quebrou). _Resultado: diretório `test/` não existe; consistente com baseline do projeto (sem testes unitários)._
- [x] 5.3 Validação manual num device/emulador: (a) escanear EAN-13 de um produto real → campo de busca preenchido + busca dispara, (b) apontar para QR de NFC-e na tela de produto → ignorado (QR não está nos formatos), (c) Code-128 alfanumérico → SnackBar de rejeição, (d) cancelar com voltar do AppBar → controller mantém valor anterior, (e) lanterna liga/desliga em ambas as telas, (f) mira aparece em ambas as telas com proporções diferentes (1:1 vs 3.5:1), (g) haptic dispara só em detecções válidas em ambas. _Validado pelo usuário no Galaxy S21 (build debug via Wi-Fi adb) — checklist aprovado._
- [x] 5.4 Verificar que `pubspec.yaml` continua sem novas dependências (`mobile_scanner` e `permission_handler` já estão listados). _Confirmado: nenhuma alteração em pubspec.yaml._

## 6. Verificação de spec e arquivamento

- [x] 6.1 Rodar `openspec verify add-items-search-barcode-scan` (ou `/opsx:verify`) e resolver eventuais lacunas entre implementação e specs. _Resultado: `openspec validate` passa; status mostra `4/4 artifacts complete`._
- [x] 6.2 Quando aprovado e implementado, executar `openspec archive add-items-search-barcode-scan` para mover delta specs para `openspec/specs/` (cria `mobile-items-search/spec.md` novo e atualiza `mobile-qrcode-capture/spec.md` com os requisitos adicionados/modificados).
