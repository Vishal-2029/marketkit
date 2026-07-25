class EarningsItem {
  final String designId;
  final String title;
  final int sales;
  final int earnedInPaise;

  const EarningsItem({
    required this.designId,
    required this.title,
    required this.sales,
    required this.earnedInPaise,
  });

  factory EarningsItem.fromJson(Map<String, dynamic> json) => EarningsItem(
        designId: json['design_id'] as String? ?? '',
        title: json['title'] as String? ?? '',
        sales: (json['sales'] as num?)?.toInt() ?? 0,
        earnedInPaise: (json['earned_in_paise'] as num?)?.toInt() ?? 0,
      );
}

class EarningsModel {
  final int totalEarnedInPaise;
  final int totalSales;
  final List<EarningsItem> items;

  const EarningsModel({
    required this.totalEarnedInPaise,
    required this.totalSales,
    required this.items,
  });

  String get formattedTotal =>
      '₹${(totalEarnedInPaise / 100).toStringAsFixed(0)}';

  factory EarningsModel.fromJson(Map<String, dynamic> json) => EarningsModel(
        totalEarnedInPaise:
            (json['total_earned_in_paise'] as num?)?.toInt() ?? 0,
        totalSales: (json['total_sales'] as num?)?.toInt() ?? 0,
        items: (json['items'] as List<dynamic>? ?? [])
            .map((e) => EarningsItem.fromJson(e as Map<String, dynamic>))
            .toList(),
      );
}
