import 'package:flutter/material.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import 'package:permission_handler/permission_handler.dart';

import 'package:jullius_scan/features/scanner/manual_entry_dialog.dart';

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
  final MobileScannerController _controller = MobileScannerController();
  bool _hasScanned = false;
  bool _cameraPermissionDenied = false;

  @override
  void initState() {
    super.initState();
    _checkCameraPermission();
  }

  Future<void> _checkCameraPermission() async {
    final status = await Permission.camera.request();
    if (!mounted) return;
    if (status.isPermanentlyDenied || status.isDenied) {
      setState(() => _cameraPermissionDenied = true);
    }
  }

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
          IconButton(
            icon: const Icon(Icons.keyboard),
            tooltip: 'Inserir URL manualmente',
            onPressed: _openManualEntry,
          ),
        ],
      ),
      body: _cameraPermissionDenied ? _buildPermissionDenied() : _buildScanner(),
    );
  }

  Widget _buildScanner() {
    return Stack(
      children: [
        MobileScanner(controller: _controller, onDetect: _onDetect),
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

  Widget _buildPermissionDenied() {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.no_photography, size: 64, color: Colors.grey),
            const SizedBox(height: 16),
            const Text(
              'Permissão de câmera negada.',
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 16),
            ),
            const SizedBox(height: 8),
            const Text(
              'Para escanear QR codes, habilite o acesso à câmera nas configurações do sistema.',
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 24),
            ElevatedButton(
              onPressed: openAppSettings,
              child: const Text('Abrir Configurações'),
            ),
          ],
        ),
      ),
    );
  }
}
