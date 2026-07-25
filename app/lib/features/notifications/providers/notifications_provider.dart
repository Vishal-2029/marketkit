import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/notification_model.dart';
import '../services/notification_api_service.dart';

class NotificationsState {
  final List<NotificationModel> notifications;
  final int unreadCount;
  final bool isLoading;

  const NotificationsState({
    this.notifications = const [],
    this.unreadCount = 0,
    this.isLoading = false,
  });

  NotificationsState copyWith({
    List<NotificationModel>? notifications,
    int? unreadCount,
    bool? isLoading,
  }) =>
      NotificationsState(
        notifications: notifications ?? this.notifications,
        unreadCount: unreadCount ?? this.unreadCount,
        isLoading: isLoading ?? this.isLoading,
      );
}

class NotificationsNotifier extends StateNotifier<NotificationsState> {
  final NotificationApiService _service;

  NotificationsNotifier(this._service) : super(const NotificationsState());

  Future<void> load() async {
    state = state.copyWith(isLoading: true);
    try {
      final result = await _service.fetchNotifications();
      state = state.copyWith(
        isLoading: false,
        notifications: result.notifications,
        unreadCount: result.unreadCount,
      );
    } catch (_) {
      state = state.copyWith(isLoading: false);
    }
  }

  Future<void> markAllRead() async {
    if (state.unreadCount == 0) return;
    try {
      await _service.markAllRead();
      state = state.copyWith(
        unreadCount: 0,
        notifications:
            state.notifications.map((n) => n.copyWith(read: true)).toList(),
      );
    } catch (_) {}
  }

  Future<void> clearAll() async {
    await _service.clearAll();
    state = state.copyWith(notifications: const [], unreadCount: 0);
  }
}

final _notificationsServiceProvider =
    Provider((ref) => NotificationApiService());

final notificationsProvider =
    StateNotifierProvider<NotificationsNotifier, NotificationsState>(
  (ref) => NotificationsNotifier(ref.read(_notificationsServiceProvider)),
);
