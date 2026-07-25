import 'product_model.dart';

class PurchaseModel {
  final String id;
  final String productId;
  final int amountInPaise;
  final DateTime? paidAt;
  final ProductModel? product;

  const PurchaseModel({
    required this.id,
    required this.productId,
    required this.amountInPaise,
    this.paidAt,
    this.product,
  });

  String get formattedAmount => '₹${(amountInPaise / 100).toStringAsFixed(0)}';

  factory PurchaseModel.fromJson(Map<String, dynamic> json) => PurchaseModel(
        id: json['id'] as String? ?? '',
        productId: json['product_id'] as String? ?? '',
        amountInPaise: (json['amount_in_paise'] as num?)?.toInt() ?? 0,
        paidAt: json['paid_at'] != null
            ? DateTime.tryParse(json['paid_at'] as String)
            : null,
        product: json['product'] != null
            ? ProductModel.fromJson(json['product'] as Map<String, dynamic>)
            : null,
      );
}
