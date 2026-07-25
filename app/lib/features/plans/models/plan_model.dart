class PlanModel {
  final String id;
  final String name;
  final String description;
  final int priceInPaise;
  final bool hasWillcom;
  final bool hasE4;
  final bool hasMecad;
  final int durationDays;
  final bool isActive;

  const PlanModel({
    required this.id,
    required this.name,
    this.description = '',
    required this.priceInPaise,
    required this.hasWillcom,
    required this.hasE4,
    required this.hasMecad,
    required this.durationDays,
    required this.isActive,
  });

  double get priceInRupees => priceInPaise / 100.0;
  String get formattedPrice => '₹${priceInRupees.toStringAsFixed(0)}';

  String get durationLabel {
    if (durationDays == 365) return '1 Year';
    if (durationDays == 30) return '1 Month';
    return '$durationDays Days';
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'name': name,
        'description': description,
        'price_in_paise': priceInPaise,
        'has_willcom': hasWillcom,
        'has_e4': hasE4,
        'has_mecad': hasMecad,
        'duration_days': durationDays,
        'is_active': isActive,
      };

  factory PlanModel.fromJson(Map<String, dynamic> json) => PlanModel(
        id: json['id'] as String,
        name: json['name'] as String,
        description: json['description'] as String? ?? '',
        priceInPaise: (json['price_in_paise'] as num).toInt(),
        hasWillcom: json['has_willcom'] as bool? ?? false,
        hasE4: json['has_e4'] as bool? ?? false,
        hasMecad: json['has_mecad'] as bool? ?? false,
        durationDays: json['duration_days'] as int? ?? 365,
        isActive: json['is_active'] as bool? ?? false,
      );
}
