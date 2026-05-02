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
  final String? barcode;

  const Item({
    required this.id,
    required this.description,
    required this.quantity,
    required this.unit,
    required this.unitPrice,
    required this.totalPrice,
    this.barcode,
  });

  factory Item.fromJson(Map<String, dynamic> json) {
    return Item(
      id: json['id'] as int,
      description: json['description'] as String,
      quantity: (json['quantity'] as num).toDouble(),
      unit: json['unit'] as String? ?? 'UN',
      unitPrice: (json['unit_price'] as num).toDouble(),
      totalPrice: (json['total_price'] as num).toDouble(),
      barcode: json['barcode'] as String?,
    );
  }
}

/// House member who originated a receipt's scraping job. Returned only by the
/// detail endpoint; the listing endpoint omits this block.
class SubmittedBy {
  final int id;
  final String name;
  final String email;

  const SubmittedBy({
    required this.id,
    required this.name,
    required this.email,
  });

  factory SubmittedBy.fromJson(Map<String, dynamic> json) {
    return SubmittedBy(
      id: json['id'] as int,
      name: json['name'] as String? ?? '',
      email: json['email'] as String? ?? '',
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
  final SubmittedBy? submittedBy;
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
    this.submittedBy,
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
      submittedBy: json['submitted_by'] != null
          ? SubmittedBy.fromJson(json['submitted_by'] as Map<String, dynamic>)
          : null,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}
