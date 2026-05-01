/// Store reference attached to each search result.
class ItemSearchStore {
  final int id;
  final String name;
  final String cnpj;

  const ItemSearchStore({
    required this.id,
    required this.name,
    required this.cnpj,
  });

  factory ItemSearchStore.fromJson(Map<String, dynamic> json) {
    return ItemSearchStore(
      id: json['id'] as int,
      name: json['name'] as String,
      cnpj: json['cnpj'] as String,
    );
  }
}

/// One aggregated item returned by the item search endpoint.
class ItemSearchResult {
  final String description;
  final String? barcode;
  final DateTime lastPurchasedAt;
  final double lastUnitPrice;
  final double lastTotalPrice;
  final double? previousUnitPrice;
  final double averageUnitPrice;
  final int purchaseCount;
  final ItemSearchStore store;
  final int receiptId;

  const ItemSearchResult({
    required this.description,
    this.barcode,
    required this.lastPurchasedAt,
    required this.lastUnitPrice,
    required this.lastTotalPrice,
    this.previousUnitPrice,
    required this.averageUnitPrice,
    required this.purchaseCount,
    required this.store,
    required this.receiptId,
  });

  factory ItemSearchResult.fromJson(Map<String, dynamic> json) {
    return ItemSearchResult(
      description: json['description'] as String,
      barcode: json['barcode'] as String?,
      lastPurchasedAt: DateTime.parse(json['last_purchased_at'] as String),
      lastUnitPrice: (json['last_unit_price'] as num).toDouble(),
      lastTotalPrice: (json['last_total_price'] as num).toDouble(),
      previousUnitPrice: json['previous_unit_price'] != null
          ? (json['previous_unit_price'] as num).toDouble()
          : null,
      averageUnitPrice: (json['average_unit_price'] as num).toDouble(),
      purchaseCount: json['purchase_count'] as int,
      store: ItemSearchStore.fromJson(json['store'] as Map<String, dynamic>),
      receiptId: json['receipt_id'] as int,
    );
  }
}

/// Envelope returned by GET /api/v1/items/search — list plus a flag indicating
/// the result was capped at the server-side limit.
class ItemSearchPage {
  final List<ItemSearchResult> items;
  final bool truncated;

  const ItemSearchPage({required this.items, required this.truncated});

  factory ItemSearchPage.fromJson(Map<String, dynamic> json) {
    final raw = (json['items'] as List?) ?? const [];
    return ItemSearchPage(
      items: raw
          .map((e) => ItemSearchResult.fromJson(e as Map<String, dynamic>))
          .toList(),
      truncated: json['truncated'] as bool? ?? false,
    );
  }
}
