import '../../../core/network/api_endpoints.dart';
import '../../../core/network/dio_client.dart';
import '../models/notification_model.dart';

class NotificationApiService {
  final _dio = DioClient().dio;

  Future<({List<NotificationModel> notifications, int unreadCount})>
      fetchNotifications() async {
    final res = await _dio.get(ApiEndpoints.notifications);
    final data = res.data['data'] as Map<String, dynamic>;
    final list = (data['notifications'] as List<dynamic>)
        .map((e) => NotificationModel.fromJson(e as Map<String, dynamic>))
        .toList();
    return (
      notifications: list,
      unreadCount: data['unread_count'] as int,
    );
  }

  Future<void> markAllRead() async {
    await _dio.post(ApiEndpoints.notificationsMarkRead);
  }

  Future<void> clearAll() async {
    await _dio.delete(ApiEndpoints.notificationsClear);
  }
}
