import 'package:marketkit/core/config/currency.dart';
class MarketPlanModel {
  final String id;
  final String name;
  final String description;
  final int priceMinor;
  final int durationDays;
  final int feeDiscountPct;
  final bool featuredSeller;
  final bool isActive;

  const MarketPlanModel({
    required this.id,
    required this.name,
    this.description = '',
    required this.priceMinor,
    required this.durationDays,
    required this.feeDiscountPct,
    required this.featuredSeller,
    required this.isActive,
  });

  double get priceMajor => Currency.toMajor(priceMinor);
  String get formattedPrice => Currency.format(priceMinor);

  String get durationLabel {
    if (durationDays == 365) return '1 Year';
    if (durationDays == 30) return '1 Month';
    return '$durationDays Days';
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'name': name,
        'description': description,
        'price_minor': priceMinor,
        'duration_days': durationDays,
        'fee_discount_pct': feeDiscountPct,
        'featured_seller': featuredSeller,
        'is_active': isActive,
      };

  factory MarketPlanModel.fromJson(Map<String, dynamic> json) =>
      MarketPlanModel(
        id: json['id'] as String,
        name: json['name'] as String,
        description: json['description'] as String? ?? '',
        priceMinor: (json['price_minor'] as num).toInt(),
        durationDays: (json['duration_days'] as num?)?.toInt() ?? 30,
        feeDiscountPct: (json['fee_discount_pct'] as num?)?.toInt() ?? 0,
        featuredSeller: json['featured_seller'] as bool? ?? false,
        isActive: json['is_active'] as bool? ?? false,
      );
}

class MarketPlanSubscriptionModel {
  final String id;
  final String planId;
  final String status;
  final DateTime? startDate;
  final DateTime? expiryDate;
  final MarketPlanModel? plan;

  const MarketPlanSubscriptionModel({
    required this.id,
    required this.planId,
    required this.status,
    this.startDate,
    this.expiryDate,
    this.plan,
  });

  bool get isActive => status == 'ACTIVE';

  factory MarketPlanSubscriptionModel.fromJson(Map<String, dynamic> json) =>
      MarketPlanSubscriptionModel(
        id: json['id'] as String? ?? '',
        planId: json['plan_id'] as String? ?? '',
        status: json['status'] as String? ?? '',
        startDate: json['start_date'] != null
            ? DateTime.tryParse(json['start_date'] as String)
            : null,
        expiryDate: json['expiry_date'] != null
            ? DateTime.tryParse(json['expiry_date'] as String)
            : null,
        plan: json['plan'] is Map<String, dynamic>
            ? MarketPlanModel.fromJson(json['plan'] as Map<String, dynamic>)
            : null,
      );
}
