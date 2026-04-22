import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

import 'package:jullius_scan/features/jobs/job_tracking_screen.dart';
import 'package:jullius_scan/features/scanner/qr_scanner_screen.dart';
import 'package:jullius_scan/models/api_error.dart';
import 'package:jullius_scan/models/receipt.dart';
import 'package:jullius_scan/services/api_client.dart';
import 'package:jullius_scan/services/auth_service.dart';
import 'package:jullius_scan/screens/receipt_detail_screen.dart';

/// Main screen showing the list of receipts for the user's house.
class HomeScreen extends StatefulWidget {
  final ApiClient apiClient;
  final AuthService authService;

  const HomeScreen({
    super.key,
    required this.apiClient,
    required this.authService,
  });

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  List<Receipt>? _receipts;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadReceipts();
  }

  Future<void> _loadReceipts() async {
    setState(() {
      _loading = true;
      _error = null;
    });

    try {
      final receipts = await widget.apiClient.listReceipts();
      if (mounted) {
        setState(() {
          _receipts = receipts;
          _loading = false;
        });
      }
    } on ApiError catch (e) {
      if (mounted) {
        setState(() {
          _error = e.isNotProvisioned
              ? 'Sua conta não está associada a nenhuma residência. Entre em contato com o administrador.'
              : e.message;
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = 'Falha ao carregar os recibos. Puxe para atualizar.';
          _loading = false;
        });
      }
    }
  }

  Future<void> _navigateToScan() async {
    // Step 1: scan QR code (or paste URL manually).
    final fiscalUrl = await Navigator.push<String>(
      context,
      MaterialPageRoute(builder: (_) => const QrScannerScreen()),
    );
    if (fiscalUrl == null || !mounted) return;

    // Step 2: submit the URL to the backend.
    try {
      final response = await widget.apiClient.submitReceipt(fiscalUrl);

      if (!mounted) return;

      // Step 3: track the job (handles captcha pause transparently).
      final completed = await Navigator.push<bool>(
        context,
        MaterialPageRoute(
          builder: (_) => JobTrackingScreen(
            jobId: response.jobId,
            apiClient: widget.apiClient,
          ),
        ),
      );
      if (completed == true) _loadReceipts();
    } on ApiError catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.message)),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Erro ao enviar nota: $e')),
      );
    }
  }

  Future<void> _navigateToDetail(Receipt receipt) async {
    final deleted = await Navigator.push<bool>(
      context,
      MaterialPageRoute(
        builder: (_) => ReceiptDetailScreen(
          apiClient: widget.apiClient,
          receiptId: receipt.id,
        ),
      ),
    );

    if (deleted == true && mounted) {
      _loadReceipts();
    }
  }

  @override
  Widget build(BuildContext context) {
    final currencyFormat = NumberFormat.currency(locale: 'pt_BR', symbol: 'R\$');
    final dateFormat = DateFormat('dd/MM/yyyy HH:mm');

    return Scaffold(
      appBar: AppBar(
        title: const Text('Jullius Scan'),
        actions: [
          IconButton(
            icon: const Icon(Icons.logout),
            tooltip: 'Sair',
            onPressed: () => widget.authService.signOut(),
          ),
        ],
      ),
      body: _buildBody(currencyFormat, dateFormat),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _navigateToScan,
        icon: const Icon(Icons.qr_code_scanner),
        label: const Text('Escanear NFC-e'),
      ),
    );
  }

  Widget _buildBody(NumberFormat currencyFormat, DateFormat dateFormat) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (_error != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.error_outline,
                  size: 48, color: Theme.of(context).colorScheme.error),
              const SizedBox(height: 16),
              Text(_error!, textAlign: TextAlign.center),
              const SizedBox(height: 16),
              FilledButton.tonal(
                onPressed: _loadReceipts,
                child: const Text('Tentar novamente'),
              ),
            ],
          ),
        ),
      );
    }

    if (_receipts == null || _receipts!.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.receipt_long,
                  size: 64,
                  color: Theme.of(context).colorScheme.onSurfaceVariant),
              const SizedBox(height: 16),
              Text(
                'Nenhum recibo ainda',
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 8),
              Text(
                'Toque em "Escanear NFC-e" para registrar sua primeira nota fiscal.',
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
              ),
            ],
          ),
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _loadReceipts,
      child: ListView.builder(
        padding: const EdgeInsets.only(bottom: 80),
        itemCount: _receipts!.length,
        itemBuilder: (context, index) {
          final receipt = _receipts![index];
          return _ReceiptCard(
            receipt: receipt,
            currencyFormat: currencyFormat,
            dateFormat: dateFormat,
            onTap: () => _navigateToDetail(receipt),
          );
        },
      ),
    );
  }
}

class _ReceiptCard extends StatelessWidget {
  final Receipt receipt;
  final NumberFormat currencyFormat;
  final DateFormat dateFormat;
  final VoidCallback onTap;

  const _ReceiptCard({
    required this.receipt,
    required this.currencyFormat,
    required this.dateFormat,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
      child: ListTile(
        leading: CircleAvatar(
          backgroundColor: Theme.of(context).colorScheme.primaryContainer,
          child: Icon(Icons.receipt,
              color: Theme.of(context).colorScheme.onPrimaryContainer),
        ),
        title: Text(
          currencyFormat.format(receipt.totalAmount),
          style: const TextStyle(fontWeight: FontWeight.bold),
        ),
        subtitle: Text(dateFormat.format(receipt.issuedAt.toLocal())),
        trailing: const Icon(Icons.chevron_right),
        onTap: onTap,
      ),
    );
  }
}
