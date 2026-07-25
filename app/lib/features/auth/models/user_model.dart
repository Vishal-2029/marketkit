import 'dart:convert';
import 'subscription_model.dart';

class UserModel {
  final String id;
  final String name;
  final String email;
  final String phone;
  final String? avatarUrl;
  final List<SubscriptionModel> subscriptions;
  final SubscriptionModel? subscription;
  // Server's source of truth for app mode — used to reconcile a device with
  // no locally-cached mode (e.g. a fresh install or new device logging into
  // an existing account) instead of wrongly re-showing the mode chooser.
  final String? currentAppMode;

  const UserModel({
    required this.id,
    required this.name,
    required this.email,
    required this.phone,
    this.avatarUrl,
    this.subscriptions = const [],
    this.subscription,
    this.currentAppMode,
  });

  List<SubscriptionModel> get activeSubscriptions =>
      subscriptions.where((s) => s.isActive).toList();

  Set<String> get activePlanIds =>
      activeSubscriptions.map((s) => s.planId).toSet();

  factory UserModel.fromJson(Map<String, dynamic> json) {
    List<SubscriptionModel> subs = [];
    if (json['subscriptions'] is List) {
      subs = (json['subscriptions'] as List)
          .map((e) => SubscriptionModel.fromJson(e as Map<String, dynamic>))
          .toList();
    } else if (json['subscription'] != null) {
      subs = [
        SubscriptionModel.fromJson(
            json['subscription'] as Map<String, dynamic>),
      ];
    }

    final legacySub = json['subscription'] != null
        ? SubscriptionModel.fromJson(
            json['subscription'] as Map<String, dynamic>)
        : (subs.isNotEmpty ? subs.last : null);

    return UserModel(
      id: json['id'] as String,
      name: json['name'] as String? ?? '',
      email: json['email'] as String,
      phone: json['phone'] as String? ?? '',
      avatarUrl: json['avatar_url'] as String?,
      subscriptions: subs,
      subscription: legacySub,
      currentAppMode: json['current_app_mode'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'name': name,
        'email': email,
        'phone': phone,
        'avatar_url': avatarUrl,
      };

  String toJsonString() => jsonEncode(toJson());
  factory UserModel.fromJsonString(String s) =>
      UserModel.fromJson(jsonDecode(s) as Map<String, dynamic>);
}
