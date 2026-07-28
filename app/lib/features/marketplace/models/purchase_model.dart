import 'product_model.dart';
import 'package:marketkit/core/config/currency.dart';

class PurchaseModel {
  final String id;
  final String productId;
  final int amountMinor;
  final DateTime? paidAt;
  final ProductModel? product;

  const PurchaseModel({
    required this.id,
    required this.productId,
    required this.amountMinor,
    this.paidAt,
    this.product,
  });

  String get formattedAmount => Currency.format(amountMinor);

  factory PurchaseModel.fromJson(Map<String, dynamic> json) => PurchaseModel(
        id: json['id'] as String? ?? '',
        productId: json['product_id'] as String? ?? '',
        amountMinor: (json['amount_minor'] as num?)?.toInt() ?? 0,
        paidAt: json['paid_at'] != null
            ? DateTime.tryParse(json['paid_at'] as String)
            : null,
        product: json['product'] != null
            ? ProductModel.fromJson(json['product'] as Map<String, dynamic>)
            : null,
      );
}
