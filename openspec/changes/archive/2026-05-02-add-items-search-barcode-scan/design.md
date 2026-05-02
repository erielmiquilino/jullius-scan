## Context

A `HomeScreen` (mobile/lib/screens/home_screen.dart) já contém uma `TextField` (dentro do widget privado `_SearchHeader`) com `prefixIcon: Icons.search` e um `suffixIcon` condicional `Icons.close` que aparece quando há texto no controller. O método `_onQueryChanged` debounce 300ms e chama `_runSearch`, que delega ao `ApiClient.searchItems(q, periodDays)`.

O backend (capability `item-search-history`) já trata `q` como busca exata por barcode quando o conteúdo é só dígitos com 8+ caracteres ("Detecção automática de busca por barcode vs descrição"). Portanto, do ponto de vista de protocolo HTTP, a feature é puramente client-side: basta colocar o EAN no campo `q`.

O app já depende de `mobile_scanner` (usado em `qr_scanner_screen.dart` para QR de NFC-e) e `permission_handler`. A `QrScannerScreen` filtra detecções pela regex `_nfceUrlPattern` — isso significa que ela rejeita um EAN puro (a string "7891234567890" não é uma URL `.gov.br`). Reaproveitar literalmente a tela existente daria UX errada (usuário escaneia produto, recebe "QR code não reconhecido como NFC-e válida").

## Goals / Non-Goals

**Goals:**
- Reduzir fricção de busca por EAN: 1 toque + apontar a câmera, em vez de digitar 13 dígitos.
- Reaproveitar pacotes nativos já configurados (`mobile_scanner`, `permission_handler`) e o tratamento de permissão de câmera já existente.
- Manter o backend e o contrato HTTP intocados — toda a mudança é na camada de apresentação.
- Diferenciar claramente, no código, "scanner de NFC-e (URL via QR)" de "scanner de produto (EAN via 1D)" para que mudanças futuras em um não afetem o outro.
- Elevar a UX das duas telas de câmera a um patamar comum (mira, lanterna, haptic), via widgets compartilhados — sem fundir as duas telas numa só.

**Non-Goals:**
- Não vamos consolidar `QrScannerScreen` e a nova `ProductBarcodeScannerScreen` numa tela genérica polivalente. Os dois fluxos têm validação, formatos suportados e ação pós-detecção diferentes; eles compartilham apenas widgets de UI (mira, torch, gate de permissão), não a screen em si.
- Não vamos adicionar um endpoint novo de "lookup por EAN" no backend — o `GET /api/v1/items/search` já cobre o caso.
- Não vamos lidar com EANs que não existam no histórico da casa: a busca simplesmente retorna 0 resultados, com a UI vazia que já existe.
- Não vamos suportar entrada manual de EAN dentro da tela do scanner (o usuário já pode digitar direto na barra de busca da `HomeScreen`); fallback é simplesmente fechar a tela.
- Não vamos validar checksum de EAN-13/EAN-8/UPC no client. Aceitamos `^\d{8,}$` (mesma regra do backend) — Code-128/ITF numéricos passam.
- Não vamos persistir o estado da lanterna entre aberturas da tela; cada vez que a tela abre, lanterna começa apagada.

## Decisions

### 1. Tela dedicada `ProductBarcodeScannerScreen` em vez de reaproveitar `QrScannerScreen`

`QrScannerScreen` filtra resultados por uma regex de URL `.gov.br` e normaliza o formato do parâmetro `p=...` da NFC-e — comportamento que não faz sentido para um EAN. Acoplar uma "modalidade" via flag levaria a `if (modeIsBarcode) ... else ...` em todo o arquivo. As telas permanecem separadas, mas compartilham widgets de UI (ver Decisão 2).

**Alternativa considerada:** uma única `ScannerScreen<T>` polivalente parametrizada por uma callback de validação/transformação. Rejeitada — adiciona generics e indireção sem ganho real, dado que existem só dois call sites e cada um tem ações pós-detecção bem distintas (`Navigator.pop(url)` que vira submit de receipt vs. `Navigator.pop(ean)` que vira filtro de busca).

### 2. Widgets compartilhados em `mobile/lib/features/scanner/widgets/`

Para honrar o pedido de "reaproveite código se possível", extraímos para `widgets/` os componentes de UI puros (sem lógica de negócio):

- **`CameraPermissionGate`** (`StatefulWidget`): encapsula a chamada `Permission.camera.request()` no `initState` e renderiza ou (a) o `child` (a câmera de fato) quando concedida, ou (b) a tela "permissão negada" com `openAppSettings`. Contrato: `CameraPermissionGate({required Widget child})`. Substitui a duplicação que existiria entre as duas telas.

- **`ScannerViewfinder`** (`StatelessWidget`): overlay sobre o `MobileScanner` com máscara escurecida (Color.black54) e janela retangular transparente no centro com cantos arredondados destacados. Contrato: `ScannerViewfinder({required double aspectRatio, double widthFraction = 0.8})`. A `QrScannerScreen` passa `aspectRatio: 1.0` (alvo quadrado para QR); a `ProductBarcodeScannerScreen` passa `aspectRatio: 3.5` (alvo wide, condizente com EAN-13).

- **Torch button**: implementado inline em cada tela como `IconButton` simples — não justifica widget próprio. Padrão:
  ```dart
  ValueListenableBuilder<TorchState>(
    valueListenable: _controller.torchState,
    builder: (_, state, __) => IconButton(
      icon: Icon(state == TorchState.on ? Icons.flash_on : Icons.flash_off),
      onPressed: _controller.toggleTorch,
    ),
  )
  ```

**Alternativa considerada:** abstrair tudo (incluindo torch) num widget `ScannerScaffold`. Rejeitada — vira "interface god widget" e o ganho é de 5 linhas. Os primeiros dois (`Gate`, `Viewfinder`) salvam código real; o torch não.

### 2. Formatos 1D suportados

`mobile_scanner` permite restringir formatos via `MobileScannerController(formats: [...])`. Para produtos brasileiros de supermercado o EAN-13 cobre praticamente 100% dos casos, mas EAN-8 (produtos pequenos), UPC-A (importados americanos), Code-128 e ITF-14 (caixas) aparecem ocasionalmente. Vamos habilitar todos os 1D comuns:

```dart
formats: [
  BarcodeFormat.ean13,
  BarcodeFormat.ean8,
  BarcodeFormat.upcA,
  BarcodeFormat.upcE,
  BarcodeFormat.code128,
  BarcodeFormat.itf,
]
```

QR (`BarcodeFormat.qrCode`) NÃO é incluído — se o usuário apontar para um QR de NFC-e nesta tela, o detector não deve enxergá-lo (evita ambiguidade com a tela de scan de NFC-e).

**Alternativa considerada:** `BarcodeFormat.all`. Rejeitada por aumentar falsos positivos (Data Matrix, PDF417 em CNHs, etc.) sem ganho real para o caso de uso de supermercado.

### 3. Validação no client antes de retornar à `HomeScreen`

A tela do scanner valida que o `rawValue` é uma string só de dígitos com 8+ caracteres antes de chamar `Navigator.pop(context, ean)`. Caso contrário, mostra um SnackBar "Código não reconhecido como EAN" e continua escaneando. Isso reflete a regra do backend ("8+ dígitos contínuos" → busca exata por barcode) e evita preencher o campo de busca com lixo que dispararia uma busca por descrição inútil.

### 4. UX do botão na barra de busca

A `TextField` tem hoje:
- `prefixIcon: Icons.search`
- `suffixIcon`: `Icons.close` quando há texto, `null` caso contrário.

Vamos mudar `suffixIcon` para um `Row(mainAxisSize: MainAxisSize.min)` que mostra:
- **Quando vazio**: apenas o `IconButton(icon: Icon(Icons.barcode_reader))` (Material Symbols tem `Icons.barcode_reader`; alternativa estável: `Icons.qr_code_scanner` já usado no FAB seria confuso; outra: `Icons.document_scanner`. **Decisão: `Icons.barcode_reader`** — é o único ícone Material Design que ilustra explicitamente um código de barras 1D, evitando confusão visual com o FAB de QR).
- **Quando há texto**: apenas o `IconButton(icon: Icon(Icons.close))` (limpar) — o botão da câmera some, porque tocar no scanner faria sentido só quando o campo está vazio (escanear depois de digitar substituiria o texto, comportamento confuso).

**Alternativa considerada:** mostrar os dois ícones simultaneamente. Rejeitada — barra de busca fica visualmente carregada e sem espaço suficiente em telas pequenas.

### 5. Fluxo de retorno do scanner

`Navigator.push<String>` retorna a string do EAN (ou `null` se cancelado). No `then`, a `HomeScreen`:
1. Define `_searchController.text = ean` (atualiza visualmente o campo).
2. Chama `_onQueryChanged(ean)` (mesma rota do digit-by-digit, dispara debounce e busca).

Isso garante que o EAN passa pelo mesmo caminho de validação (`length >= _minQueryLength`, debounce, `_runSearch`) — não há código duplicado de submissão.

### 6. Permissão de câmera

Substitui a lógica inline atual da `QrScannerScreen` pelo widget compartilhado `CameraPermissionGate` — comportamento funcional idêntico (request + UI de "negado" + link para `openAppSettings`), mas centralizado.

### 7. Feedback háptico

`HapticFeedback.lightImpact()` é disparado **uma única vez** no instante em que `_onDetect` decide aceitar a detecção (após validação). Detecções rejeitadas (rawValue inválido, ou QR que não casa `_nfceUrlPattern`) NÃO vibram — o haptic é o sinal de "li algo válido", não de "li alguma coisa".

### 8. Lanterna (torch)

Botão no `AppBar` de ambas as telas. Estado padrão: apagada ao abrir. Não persistimos preferência entre sessões (overhead de SharedPreferences não se justifica para um botão que o usuário toca em ~2s).

### 9. Overlay de mira (viewfinder)

Funções: (a) guiar o enquadramento do usuário, (b) sinalizar visualmente que aquela região é onde o detector está procurando. A janela transparente NÃO restringe o detector — o `mobile_scanner` continua escaneando o frame inteiro; o overlay é puramente visual. Restringir a área de detecção real exigiria `MobileScannerController(scanWindow: Rect)`, que adiciona complexidade de coordenadas e foi descartado para esta primeira versão.

## Risks / Trade-offs

- **[Risco] Duplicação de código entre `QrScannerScreen` e `ProductBarcodeScannerScreen` (lógica de permissão, layout de "permissão negada")** → Mitigação: aceitável neste change (~20 linhas duplicadas). Se uma terceira tela com câmera surgir, extrair `CameraPermissionGate` como widget reutilizável.

- **[Risco] Falsos positivos na detecção (câmera lê código de barras de outro produto na prateleira)** → Mitigação: o `mobile_scanner` retorna apenas códigos enquadrados na viewport ativa; o usuário tem feedback visual normal da câmera. Se virar problema na prática, podemos adicionar um overlay com "alvo" (frame retangular) — fora do escopo agora.

- **[Risco] EAN escaneado não existe no histórico da casa, usuário pensa que o scanner falhou** → Mitigação: a UI de "Nenhum item encontrado" já existe e mostra o termo buscado. O usuário verá o EAN preenchido no campo + o estado vazio, deixando claro que o scan funcionou mas não há histórico.

- **[Trade-off] Não usar `BarcodeFormat.all`** significa que tipos exóticos (Data Matrix em farmácias, PDF417 em alguns rótulos importados) não funcionam. Aceito — fora do caso de uso central (NFC-e brasileira de supermercado, onde EAN-13 domina).

- **[Trade-off] Mudar o `suffixIcon` da `TextField` para um `Row` introduz um pequeno overhead de layout** → Negligível; medições de Flutter mostram custo abaixo do limiar de 16ms mesmo em devices low-end.

## Migration Plan

Não há migração — feature aditiva, sem mudança de schema, sem mudança de protocolo. Rollback é reverter o commit; nenhum estado persistido é afetado. Sem feature flag (escopo pequeno, equipe de uma pessoa, app de uso pessoal — overhead de gating não se justifica).

## Open Questions

Nenhuma — todas as decisões UX e de escopo foram travadas com o usuário antes de virar specs:
- Ícone do botão da câmera na barra de busca: `Icons.barcode_reader`.
- Comportamento com texto presente na barra: botão da câmera some.
- Validação client-side: `^\d{8,}$`.
- Extras incluídos desde o início: lanterna, haptic, mira — também aplicados ao `QrScannerScreen` da NFC-e.
