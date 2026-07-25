class SubscriptionModel {
  final String id;
  final String planId;
  final String planName;
  final String status;
  final DateTime? expiresAt;
  final bool hasWillcom;
  final bool hasE4;
  final bool hasMecad;

  const SubscriptionModel({
    required this.id,
    required this.planId,
    required this.planName,
    required this.status,
    this.expiresAt,
    required this.hasWillcom,
    required this.hasE4,
    required this.hasMecad,
  });

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
        hasWillcom: json['has_willcom'] as bool? ?? false,
        hasE4: json['has_e4'] as bool? ?? false,
        hasMecad: json['has_mecad'] as bool? ?? false,
      );
}
