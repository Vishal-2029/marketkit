class SubscriptionModel {
  final String id;
  final String planId;
  final String planName;
  final String status;
  final DateTime? expiresAt;
  /// Feature keys granted by this subscription's plan.
  final List<String> features;

  const SubscriptionModel({
    required this.id,
    required this.planId,
    required this.planName,
    required this.status,
    this.expiresAt,
    this.features = const [],
  });

  /// Whether this subscription grants [key].
  bool hasFeature(String key) => features.contains(key);

  bool get isActive =>
      status == 'ACTIVE' &&
      (expiresAt == null || expiresAt!.isAfter(DateTime.now()));

  factory SubscriptionModel.fromJson(Map<String, dynamic> json) =>
      SubscriptionModel(
        id: json['id'] as String,
        planId: json['plan_id'] as String,
        planName: json['plan_name'] as String,
        status: json['status'] as String,
        expiresAt: json['expires_at'] != null
            ? DateTime.tryParse(json['expires_at'] as String)
            : null,
        features:
            (json['features'] as List?)?.map((e) => e as String).toList() ??
                const [],
      );
}
