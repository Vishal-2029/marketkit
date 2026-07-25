import 'design_model.dart';

class PurchaseModel {
  final String id;
  final String designId;
  final int amountInPaise;
  final DateTime? paidAt;
  final DesignModel? design;

  const PurchaseModel({
    required this.id,
    required this.designId,
    required this.amountInPaise,
    this.paidAt,
    this.design,
  });

  String get formattedAmount => '₹${(amountInPaise / 100).toStringAsFixed(0)}';

  factory PurchaseModel.fromJson(Map<String, dynamic> json) => PurchaseModel(
        id: json['id'] as String? ?? '',
        designId: json['design_id'] as String? ?? '',
        amountInPaise: (json['amount_in_paise'] as num?)?.toInt() ?? 0,
        paidAt: json['paid_at'] != null
            ? DateTime.tryParse(json['paid_at'] as String)
            : null,
        design: json['design'] != null
            ? DesignModel.fromJson(json['design'] as Map<String, dynamic>)
            : null,
      );
}
