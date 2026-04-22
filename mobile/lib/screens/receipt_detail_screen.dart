import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

import 'package:jullius_scan/models/api_error.dart';
import 'package:jullius_scan/models/receipt.dart';
import 'package:jullius_scan/services/api_client.dart';

/// Displays full receipt details including store and items.
class ReceiptDetailScreen extends StatefulWidget {
  final ApiClient apiClient;
  final int receiptId;

  const ReceiptDetailScreen({
    super.key,
    required this.apiClient,
    required this.receiptId,
  });

  @override
  State<ReceiptDetailScreen> createState() => _ReceiptDetailScreenState();
}

class _ReceiptDetailScreenState extends State<ReceiptDetailScreen> {
  Receipt? _receipt;
  bool _loading = true;
  bool _deleting = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadReceipt();
  }

  Future<void> _loadReceipt() async {
    setState(() {
      _loading = true;
      _error = null;
    });

    try {
      final receipt = await widget.apiClient.getReceipt(widget.receiptId);
      if (mounted) {
        setState(() {
          _receipt = receipt;
          _loading = false;
        });
      }
    } on ApiError catch (e) {
      if (mounted) setState(() { _error = e.message; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = 'Falha ao carregar o recibo.'; _loading = false; });
    }
  }

  Future<void> _confirmDelete() async {
    if (_receipt == null || _deleting) return;

    final confirmed = await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: const Text('Remover lançamento?'),
            content: const Text('Deseja mesmo remover este lançamento?'),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(context, false),
                child: const Text('Cancelar'),
              ),
              FilledButton(
                onPressed: () => Navigator.pop(context, true),
                child: const Text('Remover'),
              ),
            ],
          ),
        ) ??
        false;

    if (!confirmed || !mounted) return;

    await _deleteReceipt();
  }

  Future<void> _deleteReceipt() async {
    setState(() {
      _deleting = true;
    });

    try {
      await widget.apiClient.deleteReceipt(widget.receiptId);
      if (!mounted) return;
      Navigator.pop(context, true);
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _deleting = false;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.message)),
      );
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _deleting = false;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Falha ao remover o lançamento.')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Detalhes do Recibo'),
        actions: [
          if (_receipt != null)
            if (_deleting)
              const Padding(
                padding: EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                child: SizedBox(
                  width: 24,
                  height: 24,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              )
            else
              IconButton(
                onPressed: _confirmDelete,
                tooltip: 'Remover lançamento',
                icon: const Icon(Icons.delete_outline),
              ),
        ],
      ),
      body: _buildBody(context),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) return const Center(child: CircularProgressIndicator());

    if (_error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48,
                color: Theme.of(context).colorScheme.error),
            const SizedBox(height: 16),
            Text(_error!),
            const SizedBox(height: 16),
            FilledButton.tonal(
                onPressed: _loadReceipt, child: const Text('Tentar novamente')),
          ],
        ),
      );
    }

    final receipt = _receipt!;
    final currencyFormat = NumberFormat.currency(locale: 'pt_BR', symbol: 'R\$');
    final dateFormat = DateFormat('dd/MM/yyyy HH:mm');

    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Total
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  Icon(Icons.attach_money,
                      color: Theme.of(context).colorScheme.primary),
                  const SizedBox(width: 12),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Total',
                          style: Theme.of(context).textTheme.labelMedium),
                      Text(
                        currencyFormat.format(receipt.totalAmount),
                        style:
                            Theme.of(context).textTheme.headlineSmall?.copyWith(
                                  fontWeight: FontWeight.bold,
                                ),
                      ),
                    ],
                  ),
                  const Spacer(),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      Text('Emitido em',
                          style: Theme.of(context).textTheme.labelMedium),
                      Text(dateFormat.format(receipt.issuedAt.toLocal())),
                    ],
                  ),
                ],
              ),
            ),
          ),

          // Store
          if (receipt.store != null) ...[
            const SizedBox(height: 16),
            Text('Estabelecimento', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(receipt.store!.name,
                        style: const TextStyle(fontWeight: FontWeight.bold)),
                    const SizedBox(height: 4),
                    Text('CNPJ: ${receipt.store!.cnpj}',
                        style: Theme.of(context).textTheme.bodySmall),
                    if (receipt.store!.address.isNotEmpty) ...[
                      const SizedBox(height: 4),
                      Text(receipt.store!.address,
                          style: Theme.of(context).textTheme.bodySmall),
                    ],
                  ],
                ),
              ),
            ),
          ],

          // Items
          if (receipt.items != null && receipt.items!.isNotEmpty) ...[
            const SizedBox(height: 16),
            Text(
              'Itens (${receipt.items!.length})',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 8),
            Card(
              child: Column(
                children: [
                  for (var i = 0; i < receipt.items!.length; i++) ...[
                    if (i > 0) const Divider(height: 1),
                    _ItemTile(
                        item: receipt.items![i],
                        currencyFormat: currencyFormat),
                  ],
                ],
              ),
            ),
          ],

          // Fiscal key
          if (receipt.fiscalKey.isNotEmpty) ...[
            const SizedBox(height: 16),
            Text('Chave de Acesso', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            Card(
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: SelectableText(
                  receipt.fiscalKey,
                  style: Theme.of(context)
                      .textTheme
                      .bodySmall
                      ?.copyWith(fontFamily: 'monospace'),
                ),
              ),
            ),
          ],

          const SizedBox(height: 32),
        ],
      ),
    );
  }
}

class _ItemTile extends StatelessWidget {
  final Item item;
  final NumberFormat currencyFormat;

  const _ItemTile({required this.item, required this.currencyFormat});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(item.description,
                    style: const TextStyle(fontWeight: FontWeight.w500)),
                const SizedBox(height: 2),
                Text(
                  '${item.quantity} ${item.unit} x ${currencyFormat.format(item.unitPrice)}',
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                ),
                if (item.barcode != null && item.barcode!.isNotEmpty) ...[
                  const SizedBox(height: 2),
                  Text(
                    'EAN: ${item.barcode}',
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: Theme.of(context)
                              .colorScheme
                              .onSurfaceVariant
                              .withAlpha(160),
                          fontFamily: 'monospace',
                        ),
                  ),
                ],
              ],
            ),
          ),
          Text(
            currencyFormat.format(item.totalPrice),
            style: const TextStyle(fontWeight: FontWeight.bold),
          ),
        ],
      ),
    );
  }
}
