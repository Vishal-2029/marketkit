import 'package:marketkit/core/config/currency.dart';
class EarningsItem {
  final String productId;
  final String title;
  final int sales;
  final int earnedMinor;

  const EarningsItem({
    required this.productId,
    required this.title,
    required this.sales,
    required this.earnedMinor,
  });

  factory EarningsItem.fromJson(Map<String, dynamic> json) => EarningsItem(
        productId: json['product_id'] as String? ?? '',
        title: json['title'] as String? ?? '',
        sales: (json['sales'] as num?)?.toInt() ?? 0,
        earnedMinor: (json['earned_minor'] as num?)?.toInt() ?? 0,
      );
}

class EarningsModel {
  final int totalEarnedMinor;
  final int totalSales;
  final List<EarningsItem> items;

  const EarningsModel({
    required this.totalEarnedMinor,
    required this.totalSales,
    required this.items,
  });

  String get formattedTotal =>
      Currency.format(totalEarnedMinor);

  factory EarningsModel.fromJson(Map<String, dynamic> json) => EarningsModel(
        totalEarnedMinor:
            (json['total_earned_minor'] as num?)?.toInt() ?? 0,
        totalSales: (json['total_sales'] as num?)?.toInt() ?? 0,
        items: (json['items'] as List<dynamic>? ?? [])
            .map((e) => EarningsItem.fromJson(e as Map<String, dynamic>))
            .toList(),
      );
}
