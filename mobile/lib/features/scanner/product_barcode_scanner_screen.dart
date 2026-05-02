import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:mobile_scanner/mobile_scanner.dart';

import 'package:jullius_scan/features/scanner/widgets/camera_permission_gate.dart';
import 'package:jullius_scan/features/scanner/widgets/scanner_viewfinder.dart';

final _eanPattern = RegExp(r'^\d{8,}$');

class ProductBarcodeScannerScreen extends StatefulWidget {
  const ProductBarcodeScannerScreen({super.key});

  @override
  State<ProductBarcodeScannerScreen> createState() =>
      _ProductBarcodeScannerScreenState();
}

class _ProductBarcodeScannerScreenState
    extends State<ProductBarcodeScannerScreen> {
  final MobileScannerController _controller = MobileScannerController(
    formats: const [
      BarcodeFormat.ean13,
      BarcodeFormat.ean8,
      BarcodeFormat.upcA,
      BarcodeFormat.upcE,
      BarcodeFormat.code128,
      BarcodeFormat.itf,
    ],
  );
  bool _hasScanned = false;
  DateTime _lastRejectionAt = DateTime.fromMillisecondsSinceEpoch(0);

  void _onDetect(BarcodeCapture capture) {
    if (_hasScanned) return;
    for (final barcode in capture.barcodes) {
      final raw = barcode.rawValue;
      if (raw == null) continue;

      if (_eanPattern.hasMatch(raw)) {
        setState(() => _hasScanned = true);
        HapticFeedback.lightImpact();
        Navigator.of(context).pop(raw);
        return;
      }

      // Throttle rejection SnackBars: detector fires at high frequency, and
      // a Code-128 alphanumeric in view would otherwise spam the user.
      final now = DateTime.now();
      if (now.difference(_lastRejectionAt) > const Duration(seconds: 2)) {
        _lastRejectionAt = now;
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Código não reconhecido como EAN.'),
            duration: Duration(seconds: 2),
          ),
        );
      }
      return;
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
        title: const Text('Escanear código de barras'),
        actions: [_TorchButton(controller: _controller)],
      ),
      body: CameraPermissionGate(child: _buildScanner()),
    );
  }

  Widget _buildScanner() {
    return Stack(
      children: [
        MobileScanner(controller: _controller, onDetect: _onDetect),
        const Positioned.fill(
          child: ScannerViewfinder(aspectRatio: 3.5),
        ),
        Positioned(
          bottom: 32,
          left: 16,
          right: 16,
          child: Center(
            child: Container(
              padding:
                  const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              decoration: BoxDecoration(
                color: Colors.black54,
                borderRadius: BorderRadius.circular(20),
              ),
              child: const Text(
                'Aponte para o código de barras do produto',
                style: TextStyle(color: Colors.white),
                textAlign: TextAlign.center,
              ),
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
