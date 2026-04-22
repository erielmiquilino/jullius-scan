import 'package:flutter/material.dart';

class ManualEntryDialog extends StatefulWidget {
  const ManualEntryDialog({super.key});

  @override
  State<ManualEntryDialog> createState() => _ManualEntryDialogState();
}

class _ManualEntryDialogState extends State<ManualEntryDialog> {
  final _controller = TextEditingController();
  String? _error;

  void _submit() {
    final url = _controller.text.trim();
    if (url.isEmpty) {
      setState(() => _error = 'Informe uma URL.');
      return;
    }
    final parsed = Uri.tryParse(url);
    if (parsed == null || !parsed.hasScheme || parsed.host.isEmpty) {
      setState(() => _error = 'URL inválida.');
      return;
    }
    Navigator.of(context).pop(url);
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Inserir URL da NFC-e'),
      content: TextField(
        controller: _controller,
        autofocus: true,
        keyboardType: TextInputType.url,
        decoration: InputDecoration(
          hintText: 'https://www.sefaz...',
          errorText: _error,
        ),
        onSubmitted: (_) => _submit(),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Cancelar'),
        ),
        ElevatedButton(
          onPressed: _submit,
          child: const Text('Enviar'),
        ),
      ],
    );
  }
}
