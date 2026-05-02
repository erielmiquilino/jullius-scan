import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:mobile_scanner/mobile_scanner.dart';

import 'package:jullius_scan/features/scanner/manual_entry_dialog.dart';
import 'package:jullius_scan/features/scanner/widgets/camera_permission_gate.dart';
import 'package:jullius_scan/features/scanner/widgets/scanner_viewfinder.dart';

// NFC-e fiscal URL pattern: any *.gov.br domain containing nfce or nfe in path/domain.
// Brazilian states use varied SEFAZ domains (e.g. sat.sef.sc.gov.br, sefaz.rs.gov.br).
final _nfceUrlPattern = RegExp(
  r'^https?://[^/]*\.gov\.br[/?].*(?:nfce|nfe)|^https?://[^/]*(?:nfce|nfe)[^/]*\.gov\.br',
  caseSensitive: false,
);

// NFC-e QR code URLs use ?p=CHAVE44|param1|param2[|extra_params...]
// Some PDVs generate extended formats with extra fields (including a wrong
// DigestValue), causing error 227 on the SEFAZ. The correct short form is
// always CHAVE|lastShort|lastSingle — the final two short-integer segments
// are the valid tpAmb/cDest pair and the extra middle params are dropped.
//
// Example (broken): CHAVE|2|1|21|27.98|hash|3|1  →  CHAVE|3|1
// Example (ok):     CHAVE|3|1                    →  CHAVE|3|1 (unchanged)
String _normalizeNfceUrl(String rawUrl) {
  try {
    final uri = Uri.parse(rawUrl);
    final p = uri.queryParameters['p'];
    if (p == null || !p.contains('|')) return rawUrl;

    final parts = p.split('|');
    final chave = parts.first.replaceAll(RegExp(r'\s'), '');
    if (chave.length != 44 || !RegExp(r'^\d{44}$').hasMatch(chave)) {
      return rawUrl;
    }

    // Already in short form — nothing to fix.
    if (parts.length <= 3) return rawUrl;

    // Extended format: reconstruct using the last two short-integer segments.
    final last = parts.last.trim();
    final secondLast = parts[parts.length - 2].trim();
    if (RegExp(r'^\d{1,2}$').hasMatch(secondLast) &&
        RegExp(r'^\d$').hasMatch(last)) {
      return uri
          .replace(queryParameters: {'p': '$chave|$secondLast|$last'})
          .toString();
    }

    // Fallback: just the chave.
    return uri.replace(queryParameters: {'p': chave}).toString();
  } catch (_) {
    return rawUrl;
  }
}

class QrScannerScreen extends StatefulWidget {
  const QrScannerScreen({super.key});

  @override
  State<QrScannerScreen> createState() => _QrScannerScreenState();
}

class _QrScannerScreenState extends State<QrScannerScreen> {
  final MobileScannerController _controller = MobileScannerController(
    formats: const [BarcodeFormat.qrCode],
  );
  bool _hasScanned = false;

  void _onDetect(BarcodeCapture capture) {
    if (_hasScanned) return;
    final barcodes = capture.barcodes;
    for (final barcode in barcodes) {
      final rawValue = barcode.rawValue;
      if (rawValue == null) continue;

      if (!_nfceUrlPattern.hasMatch(rawValue)) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('QR code não reconhecido como NFC-e válida.')),
        );
        return;
      }

      setState(() => _hasScanned = true);
      HapticFeedback.lightImpact();
      Navigator.of(context).pop(_normalizeNfceUrl(rawValue));
      return;
    }
  }

  Future<void> _openManualEntry() async {
    final url = await showDialog<String>(
      context: context,
      builder: (_) => const ManualEntryDialog(),
    );
    if (url != null && mounted) {
      Navigator.of(context).pop(_normalizeNfceUrl(url));
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Escanear NFC-e'),
        actions: [
          _TorchButton(controller: _controller),
          IconButton(
            icon: const Icon(Icons.keyboard),
            tooltip: 'Inserir URL manualmente',
            onPressed: _openManualEntry,
          ),
        ],
      ),
      body: CameraPermissionGate(child: _buildScanner()),
    );
  }

  Widget _buildScanner() {
    return Stack(
      children: [
        MobileScanner(controller: _controller, onDetect: _onDetect),
        const Positioned.fill(
          child: ScannerViewfinder(aspectRatio: 1.0),
        ),
        Positioned(
          bottom: 32,
          left: 0,
          right: 0,
          child: Center(
            child: TextButton.icon(
              icon: const Icon(Icons.keyboard_alt_outlined, color: Colors.white),
              label: const Text(
                'Inserir URL manualmente',
                style: TextStyle(color: Colors.white),
              ),
              onPressed: _openManualEntry,
            ),
          ),
        ),
      ],
    );
  }
}

class _TorchButton extends StatelessWidget {
  final MobileScannerController controller;

  const _TorchButton({required this.controller});

  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder<MobileScannerState>(
      valueListenable: controller,
      builder: (_, state, _) {
        switch (state.torchState) {
          case TorchState.unavailable:
            return const SizedBox.shrink();
          case TorchState.on:
          case TorchState.auto:
            return IconButton(
              icon: const Icon(Icons.flash_on),
              tooltip: 'Desligar lanterna',
              onPressed: controller.toggleTorch,
            );
          case TorchState.off:
            return IconButton(
              icon: const Icon(Icons.flash_off),
              tooltip: 'Ligar lanterna',
              onPressed: controller.toggleTorch,
            );
        }
      },
    );
  }
}
