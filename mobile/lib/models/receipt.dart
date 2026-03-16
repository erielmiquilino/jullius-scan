/// Store information from a receipt.
class Store {
  final int id;
  final String cnpj;
  final String name;
  final String address;

  const Store({
    required this.id,
    required this.cnpj,
    required this.name,
    this.address = '',
  });

  factory Store.fromJson(Map<String, dynamic> json) {
    return Store(
      id: json['id'] as int,
      cnpj: json['cnpj'] as String,
      name: json['name'] as String,
      address: json['address'] as String? ?? '',
    );
  }
}

/// A single item from a receipt.
class Item {
  final int id;
  final String description;
  final double quantity;
  final String unit;
  final double unitPrice;
  final double totalPrice;

  const Item({
    required this.id,
    required this.description,
    required this.quantity,
    required this.unit,
    required this.unitPrice,
    required this.totalPrice,
  });

  factory Item.fromJson(Map<String, dynamic> json) {
    return Item(
      id: json['id'] as int,
      description: json['description'] as String,
      quantity: (json['quantity'] as num).toDouble(),
      unit: json['unit'] as String? ?? 'UN',
      unitPrice: (json['unit_price'] as num).toDouble(),
      totalPrice: (json['total_price'] as num).toDouble(),
    );
  }
}

/// A parsed fiscal receipt.
class Receipt {
  final int id;
  final int houseId;
  final String fiscalKey;
  final String fiscalUrl;
  final DateTime issuedAt;
  final double totalAmount;
  final Store? store;
  final List<Item>? items;
  final DateTime createdAt;

  const Receipt({
    required this.id,
    required this.houseId,
    required this.fiscalKey,
    required this.fiscalUrl,
    required this.issuedAt,
    required this.totalAmount,
    this.store,
    this.items,
    required this.createdAt,
  });

  factory Receipt.fromJson(Map<String, dynamic> json) {
    return Receipt(
      id: json['id'] as int,
      houseId: json['house_id'] as int,
      fiscalKey: json['fiscal_key'] as String? ?? '',
      fiscalUrl: json['fiscal_url'] as String,
      issuedAt: DateTime.parse(json['issued_at'] as String),
      totalAmount: (json['total_amount'] as num).toDouble(),
      store: json['store'] != null
          ? Store.fromJson(json['store'] as Map<String, dynamic>)
          : null,
      items: json['items'] != null
          ? (json['items'] as List)
              .map((e) => Item.fromJson(e as Map<String, dynamic>))
              .toList()
          : null,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}
