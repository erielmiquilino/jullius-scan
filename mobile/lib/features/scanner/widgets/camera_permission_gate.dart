import 'package:flutter/material.dart';
import 'package:permission_handler/permission_handler.dart';

class CameraPermissionGate extends StatefulWidget {
  final Widget child;

  const CameraPermissionGate({super.key, required this.child});

  @override
  State<CameraPermissionGate> createState() => _CameraPermissionGateState();
}

class _CameraPermissionGateState extends State<CameraPermissionGate> {
  bool? _granted;

  @override
  void initState() {
    super.initState();
    _request();
  }

  Future<void> _request() async {
    final status = await Permission.camera.request();
    if (!mounted) return;
    setState(() => _granted = status.isGranted);
  }

  @override
  Widget build(BuildContext context) {
    if (_granted == null) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_granted == false) {
      return _PermissionDenied(onRetry: _request);
    }
    return widget.child;
  }
}

class _PermissionDenied extends StatelessWidget {
  final VoidCallback onRetry;

  const _PermissionDenied({required this.onRetry});

  @override
  Widget build(BuildContext context) {
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
              'Para usar o scanner, habilite o acesso à câmera nas configurações do sistema.',
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 24),
            ElevatedButton(
              onPressed: openAppSettings,
              child: const Text('Abrir Configurações'),
            ),
            const SizedBox(height: 8),
            TextButton(
              onPressed: onRetry,
              child: const Text('Tentar novamente'),
            ),
          ],
        ),
      ),
    );
  }
}
