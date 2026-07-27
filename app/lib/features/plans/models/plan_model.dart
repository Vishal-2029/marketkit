class PlanModel {
  final String id;
  final String name;
  final String description;
  final int priceInPaise;
  /// Feature keys this plan grants (e.g. video category keys).
  final List<String> features;
  final int durationDays;
  final bool isActive;

  const PlanModel({
    required this.id,
    required this.name,
    this.description = '',
    required this.priceInPaise,
    this.features = const [],
    required this.durationDays,
    required this.isActive,
  });

  /// Whether this plan grants [key].
  bool hasFeature(String key) => features.contains(key);

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
        'features': features,
        'duration_days': durationDays,
        'is_active': isActive,
      };

  factory PlanModel.fromJson(Map<String, dynamic> json) => PlanModel(
        id: json['id'] as String,
        name: json['name'] as String,
        description: json['description'] as String? ?? '',
        priceInPaise: (json['price_in_paise'] as num).toInt(),
        features:
            (json['features'] as List?)?.map((e) => e as String).toList() ??
                const [],
        durationDays: json['duration_days'] as int? ?? 365,
        isActive: json['is_active'] as bool? ?? false,
      );
}
