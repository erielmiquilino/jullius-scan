import 'dart:async';

import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

import 'package:jullius_scan/features/jobs/job_tracking_screen.dart';
import 'package:jullius_scan/features/scanner/product_barcode_scanner_screen.dart';
import 'package:jullius_scan/features/scanner/qr_scanner_screen.dart';
import 'package:jullius_scan/models/api_error.dart';
import 'package:jullius_scan/models/item_search_result.dart';
import 'package:jullius_scan/models/receipt.dart';
import 'package:jullius_scan/services/api_client.dart';
import 'package:jullius_scan/services/auth_service.dart';
import 'package:jullius_scan/screens/receipt_detail_screen.dart';

/// Period option presented as a FilterChip. A null [days] value means "all time".
class _PeriodOption {
  final int? days;
  final String label;
  const _PeriodOption(this.days, this.label);
}

const List<_PeriodOption> _periodOptions = [
  _PeriodOption(30, '30d'),
  _PeriodOption(90, '90d'),
  _PeriodOption(180, '6m'),
  _PeriodOption(null, 'Tudo'),
];

const Duration _searchDebounce = Duration(milliseconds: 300);
const int _minQueryLength = 3;

/// Main screen showing the list of receipts for the user's house. When the
/// SearchBar has at least [_minQueryLength] characters, the receipt list is
/// replaced by aggregated item search results.
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
  // Receipts list state
  List<Receipt>? _receipts;
  bool _loading = true;
  String? _error;

  // Search state
  final TextEditingController _searchController = TextEditingController();
  Timer? _debounceTimer;
  String _query = '';
  int? _periodDays = 30;
  List<ItemSearchResult>? _searchResults;
  bool _searchTruncated = false;
  bool _searchLoading = false;
  String? _searchError;
  int _searchRequestId = 0;

  bool get _searchActive => _query.trim().length >= _minQueryLength;

  @override
  void initState() {
    super.initState();
    _loadReceipts();
  }

  @override
  void dispose() {
    _debounceTimer?.cancel();
    _searchController.dispose();
    super.dispose();
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

  void _onQueryChanged(String value) {
    setState(() {
      _query = value;
    });
    _debounceTimer?.cancel();

    final trimmed = value.trim();
    if (trimmed.length < _minQueryLength) {
      setState(() {
        _searchResults = null;
        _searchError = null;
        _searchLoading = false;
        _searchTruncated = false;
      });
      return;
    }

    _debounceTimer = Timer(_searchDebounce, _runSearch);
  }

  void _onPeriodChanged(int? days) {
    setState(() {
      _periodDays = days;
    });
    if (_searchActive) {
      _runSearch();
    }
  }

  Future<void> _runSearch() async {
    final trimmed = _query.trim();
    if (trimmed.length < _minQueryLength) return;

    final requestId = ++_searchRequestId;
    setState(() {
      _searchLoading = true;
      _searchError = null;
    });

    try {
      final page = await widget.apiClient.searchItems(
        trimmed,
        periodDays: _periodDays,
      );
      if (!mounted || requestId != _searchRequestId) return;
      setState(() {
        _searchResults = page.items;
        _searchTruncated = page.truncated;
        _searchLoading = false;
      });
    } on ApiError catch (e) {
      if (!mounted || requestId != _searchRequestId) return;
      setState(() {
        _searchError = e.message;
        _searchLoading = false;
      });
    } catch (_) {
      if (!mounted || requestId != _searchRequestId) return;
      setState(() {
        _searchError = 'Falha ao buscar itens.';
        _searchLoading = false;
      });
    }
  }

  Future<void> _onScanBarcode() async {
    final ean = await Navigator.push<String>(
      context,
      MaterialPageRoute(builder: (_) => const ProductBarcodeScannerScreen()),
    );
    if (ean == null || !mounted) return;
    _searchController.text = ean;
    _onQueryChanged(ean);
  }

  Future<void> _navigateToScan() async {
    final fiscalUrl = await Navigator.push<String>(
      context,
      MaterialPageRoute(builder: (_) => const QrScannerScreen()),
    );
    if (fiscalUrl == null || !mounted) return;

    try {
      final response = await widget.apiClient.submitReceipt(fiscalUrl);

      if (!mounted) return;

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

  Future<void> _navigateToReceiptById(int receiptId) async {
    await Navigator.push(
      context,
      MaterialPageRoute(
        builder: (_) => ReceiptDetailScreen(
          apiClient: widget.apiClient,
          receiptId: receiptId,
        ),
      ),
    );
    if (mounted) {
      // Receipt may have been deleted while in detail screen — refresh quietly.
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
      body: Column(
        children: [
          _SearchHeader(
            controller: _searchController,
            onChanged: _onQueryChanged,
            onScanBarcode: _onScanBarcode,
            periodOptions: _periodOptions,
            selectedPeriodDays: _periodDays,
            onPeriodSelected: _onPeriodChanged,
            showPeriodChips: _searchActive,
          ),
          Expanded(child: _buildBody(currencyFormat, dateFormat)),
        ],
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _navigateToScan,
        icon: const Icon(Icons.qr_code_scanner),
        label: const Text('Escanear NFC-e'),
      ),
    );
  }

  Widget _buildBody(NumberFormat currencyFormat, DateFormat dateFormat) {
    if (_searchActive) {
      return _buildSearchResults(currencyFormat);
    }
    return _buildReceiptsList(currencyFormat, dateFormat);
  }

  Widget _buildReceiptsList(NumberFormat currencyFormat, DateFormat dateFormat) {
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

  Widget _buildSearchResults(NumberFormat currencyFormat) {
    if (_searchLoading && _searchResults == null) {
      return const Center(child: CircularProgressIndicator());
    }

    if (_searchError != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.error_outline,
                  size: 48, color: Theme.of(context).colorScheme.error),
              const SizedBox(height: 16),
              Text(_searchError!, textAlign: TextAlign.center),
              const SizedBox(height: 16),
              FilledButton.tonal(
                onPressed: _runSearch,
                child: const Text('Tentar novamente'),
              ),
            ],
          ),
        ),
      );
    }

    final results = _searchResults ?? const [];
    if (results.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.search_off,
                  size: 64,
                  color: Theme.of(context).colorScheme.onSurfaceVariant),
              const SizedBox(height: 16),
              Text(
                'Nenhum item encontrado',
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 8),
              Text(
                _periodDays == null
                    ? 'Tente outro termo de busca.'
                    : 'Tente outro termo ou amplie o período.',
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

    return ListView.builder(
      padding: const EdgeInsets.only(bottom: 80),
      itemCount: results.length + (_searchTruncated ? 1 : 0),
      itemBuilder: (context, index) {
        if (_searchTruncated && index == 0) {
          return Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
            child: Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: Theme.of(context).colorScheme.secondaryContainer,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Icon(Icons.info_outline,
                      size: 18,
                      color:
                          Theme.of(context).colorScheme.onSecondaryContainer),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Mostrando 50 — refine a busca para ver mais.',
                      style: Theme.of(context).textTheme.bodySmall?.copyWith(
                            color: Theme.of(context)
                                .colorScheme
                                .onSecondaryContainer,
                          ),
                    ),
                  ),
                ],
              ),
            ),
          );
        }
        final result = results[_searchTruncated ? index - 1 : index];
        return _ItemSearchResultCard(
          result: result,
          currencyFormat: currencyFormat,
          onTap: () => _navigateToReceiptById(result.receiptId),
        );
      },
    );
  }
}

class _SearchHeader extends StatelessWidget {
  final TextEditingController controller;
  final ValueChanged<String> onChanged;
  final VoidCallback onScanBarcode;
  final List<_PeriodOption> periodOptions;
  final int? selectedPeriodDays;
  final ValueChanged<int?> onPeriodSelected;
  final bool showPeriodChips;

  const _SearchHeader({
    required this.controller,
    required this.onChanged,
    required this.onScanBarcode,
    required this.periodOptions,
    required this.selectedPeriodDays,
    required this.onPeriodSelected,
    required this.showPeriodChips,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          TextField(
            controller: controller,
            onChanged: onChanged,
            decoration: InputDecoration(
              hintText: 'Buscar item ou código de barras',
              prefixIcon: const Icon(Icons.search),
              suffixIcon: controller.text.isEmpty
                  ? IconButton(
                      icon: const Icon(Icons.barcode_reader),
                      tooltip: 'Escanear código de barras',
                      onPressed: onScanBarcode,
                    )
                  : IconButton(
                      icon: const Icon(Icons.close),
                      tooltip: 'Limpar',
                      onPressed: () {
                        controller.clear();
                        onChanged('');
                      },
                    ),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(28),
                borderSide: BorderSide.none,
              ),
              filled: true,
              contentPadding:
                  const EdgeInsets.symmetric(horizontal: 16, vertical: 0),
            ),
          ),
          if (showPeriodChips) ...[
            const SizedBox(height: 8),
            Row(
              children: [
                for (int i = 0; i < periodOptions.length; i++) ...[
                  if (i > 0) const SizedBox(width: 8),
                  Expanded(
                    child: FilterChip(
                      label: Center(child: Text(periodOptions[i].label)),
                      selected: periodOptions[i].days == selectedPeriodDays,
                      onSelected: (_) =>
                          onPeriodSelected(periodOptions[i].days),
                      materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                      visualDensity: VisualDensity.compact,
                    ),
                  ),
                ],
              ],
            ),
          ],
        ],
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

class _ItemSearchResultCard extends StatelessWidget {
  final ItemSearchResult result;
  final NumberFormat currencyFormat;
  final VoidCallback onTap;

  const _ItemSearchResultCard({
    required this.result,
    required this.currencyFormat,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final dateFormat = DateFormat('dd/MM/yyyy');
    final last = dateFormat.format(result.lastPurchasedAt.toLocal());

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: Text(
                      result.description,
                      style: const TextStyle(fontWeight: FontWeight.w600),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Text(
                    currencyFormat.format(result.lastUnitPrice),
                    style: const TextStyle(
                      fontWeight: FontWeight.bold,
                      fontSize: 16,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 4),
              Row(
                children: [
                  Icon(Icons.store,
                      size: 14, color: theme.colorScheme.onSurfaceVariant),
                  const SizedBox(width: 4),
                  Expanded(
                    child: Text(
                      result.store.name,
                      style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  Icon(Icons.calendar_today,
                      size: 12, color: theme.colorScheme.onSurfaceVariant),
                  const SizedBox(width: 4),
                  Text(
                    last,
                    style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant),
                  ),
                ],
              ),
              const SizedBox(height: 6),
              Row(
                children: [
                  _Chip(
                    icon: Icons.shopping_cart,
                    label: result.purchaseCount == 1
                        ? '1 compra'
                        : '${result.purchaseCount} compras',
                  ),
                  const SizedBox(width: 6),
                  _Chip(
                    icon: Icons.show_chart,
                    label:
                        'média ${currencyFormat.format(result.averageUnitPrice)}',
                  ),
                  const SizedBox(width: 6),
                  _PriceVariationChip(
                    last: result.lastUnitPrice,
                    previous: result.previousUnitPrice,
                    currencyFormat: currencyFormat,
                  ),
                ],
              ),
              if (result.barcode != null && result.barcode!.isNotEmpty) ...[
                const SizedBox(height: 6),
                Text(
                  'EAN: ${result.barcode}',
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant.withAlpha(160),
                    fontFamily: 'monospace',
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _Chip extends StatelessWidget {
  final IconData icon;
  final String label;

  const _Chip({required this.icon, required this.label});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 12, color: theme.colorScheme.onSurfaceVariant),
          const SizedBox(width: 4),
          Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
                fontWeight: FontWeight.w500),
          ),
        ],
      ),
    );
  }
}

class _PriceVariationChip extends StatelessWidget {
  final double last;
  final double? previous;
  final NumberFormat currencyFormat;

  const _PriceVariationChip({
    required this.last,
    required this.previous,
    required this.currencyFormat,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (previous == null) {
      return _Chip(icon: Icons.remove, label: '—');
    }
    final diff = last - previous!;
    if (diff == 0) {
      return _Chip(icon: Icons.remove, label: '=');
    }
    final isUp = diff > 0;
    final color = isUp ? Colors.red.shade700 : Colors.green.shade700;
    final bg = isUp ? Colors.red.shade50 : Colors.green.shade50;
    final icon = isUp ? Icons.arrow_upward : Icons.arrow_downward;
    final label = currencyFormat.format(diff.abs());

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: theme.brightness == Brightness.dark
            ? color.withValues(alpha: 0.15)
            : bg,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 12, color: color),
          const SizedBox(width: 4),
          Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(
                color: color, fontWeight: FontWeight.w600),
          ),
        ],
      ),
    );
  }
}
