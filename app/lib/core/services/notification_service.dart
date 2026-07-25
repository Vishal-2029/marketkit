import 'dart:io' show Platform;
import 'dart:typed_data';

import 'package:app_settings/app_settings.dart';
import 'package:dio/dio.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart' show VoidCallback, debugPrint, kIsWeb;
import 'package:flutter/widgets.dart' show WidgetsBinding;
import 'package:flutter_local_notifications/flutter_local_notifications.dart';

import '../network/api_endpoints.dart';
import '../network/dio_client.dart';
import '../router/app_router.dart';
import 'web_notifications.dart';

final _localNotifications = FlutterLocalNotificationsPlugin();

const _androidChannel = AndroidNotificationChannel(
  'default_channel',
  'General Notifications',
  importance: Importance.high,
  playSound: true,
);

class NotificationService {
  static final NotificationService _instance = NotificationService._();
  NotificationService._();
  factory NotificationService() => _instance;

  /// Set by the home screen so the bell-icon badge refreshes the moment a
  /// push arrives while the app is in the foreground, instead of only on
  /// next screen mount.
  static VoidCallback? onMessageReceived;

  final _messaging = FirebaseMessaging.instance;

  void _navigateToScreen(String? screen) {
    switch (screen) {
      case 'gallery':
        appRouter.go('/gallery');
        break;
      case 'community':
        appRouter.go('/community');
        break;
      case 'profile':
        appRouter.go('/profile');
        break;
      case 'home':
        appRouter.go('/home');
        break;
      default:
        // No explicit screen — every per-user notification (Product Sold,
        // Payout Sent, purchase-thread replies, subscription updates, ...)
        // sends none. Forcing '/home' here used to hijack cold starts:
        // tapping one of these to reopen the app fired before (or right
        // after) SplashScreen's own mode-aware landing navigation, always
        // dumping a Product Market user into Learning regardless of their
        // last-used mode. Do nothing instead and let the normal splash/auth
        // flow decide where to land.
        break;
    }
  }

  Future<void> init() async {
    if (kIsWeb) {
      await _initWeb();
      return;
    }

    try {
      await _initMobile();
    } catch (e, st) {
      // unawaited(NotificationService().init()) at the call site only catches a
      // *synchronous* throw before the first await — anything failing after that
      // (permission requests, plugin init) would otherwise vanish as an unhandled
      // async error. Catch everything here so a failure is always visible.
      debugPrint('[FCM] init failed: $e\n$st');
    }
  }

  Future<void> _initMobile() async {
    await _localNotifications.initialize(
      const InitializationSettings(
        android: AndroidInitializationSettings('@mipmap/launcher_icon'),
        iOS: DarwinInitializationSettings(),
      ),
      onDidReceiveNotificationResponse: (details) =>
          _navigateToScreen(details.payload),
    );

    await _localNotifications
        .resolvePlatformSpecificImplementation<
            AndroidFlutterLocalNotificationsPlugin>()
        ?.createNotificationChannel(_androidChannel);

    // Android 13+ requires a runtime POST_NOTIFICATIONS grant (FCM requestPermission is iOS-only).
    // A denial here only affects whether local heads-up notifications can be shown —
    // the FCM token is independent of it, so we still register it below (rather than
    // bailing out) so the server can push once the user enables permission later.
    if (Platform.isAndroid) {
      final granted = await _localNotifications
          .resolvePlatformSpecificImplementation<
              AndroidFlutterLocalNotificationsPlugin>()
          ?.requestNotificationsPermission();
      if (granted == false) {
        debugPrint('[FCM] Android notification permission denied — registering token anyway');
      }
    }

    // iOS: show notifications while app is in foreground
    await _messaging.setForegroundNotificationPresentationOptions(
      alert: true,
      badge: true,
      sound: true,
    );

    final settings = await _messaging.requestPermission(
      alert: true,
      badge: true,
      sound: true,
    );

    if (settings.authorizationStatus == AuthorizationStatus.denied) {
      debugPrint('[FCM] iOS notification permission denied — registering token anyway');
    }

    try {
      final token = await _messaging.getToken();
      if (token != null) {
        await _registerToken(token);
      } else {
        debugPrint('[FCM] getToken() returned null');
      }
    } catch (e) {
      debugPrint('[FCM] getToken() failed: $e');
    }

    _messaging.onTokenRefresh.listen(_registerToken);

    // Foreground message handler — show a local notification, with a
    // big-picture image when the payload carries one.
    FirebaseMessaging.onMessage.listen((message) async {
      final n = message.notification;
      final title = n?.title ?? message.data['title'] as String?;
      final body = n?.body ?? message.data['body'] as String?;
      if (title == null && body == null) return;

      final imageUrl =
          n?.android?.imageUrl ?? message.data['image'] as String?;

      StyleInformation? style;
      AndroidBitmap<Object>? largeIcon;
      if (imageUrl != null && imageUrl.isNotEmpty) {
        try {
          // Plain Dio (no auth interceptor) — the image is on a public CDN.
          final res = await Dio().get<List<int>>(
            imageUrl,
            options: Options(responseType: ResponseType.bytes),
          );
          final bmp = ByteArrayAndroidBitmap(Uint8List.fromList(res.data!));
          largeIcon = bmp;
          style = BigPictureStyleInformation(
            bmp,
            largeIcon: bmp,
            contentTitle: title,
            summaryText: body,
          );
        } catch (e) {
          debugPrint('[FCM] notification image fetch failed: $e');
        }
      }

      _localNotifications.show(
        message.hashCode,
        title,
        body,
        NotificationDetails(
          android: AndroidNotificationDetails(
            _androidChannel.id,
            _androidChannel.name,
            importance: Importance.high,
            priority: Priority.high,
            playSound: true,
            styleInformation: style,
            largeIcon: largeIcon,
          ),
          iOS: const DarwinNotificationDetails(
            presentAlert: true,
            presentBadge: true,
            presentSound: true,
          ),
        ),
        payload: message.data['screen'] as String?,
      );

      onMessageReceived?.call();
    });

    // Handle notification tap when app was in background (not terminated)
    FirebaseMessaging.onMessageOpenedApp.listen(
      (message) => _navigateToScreen(message.data['screen'] as String?),
    );

    // Handle notification tap when app was terminated — delay navigation until
    // after the first frame so the router is attached to the widget tree.
    final initialMessage = await _messaging.getInitialMessage();
    if (initialMessage != null) {
      WidgetsBinding.instance.addPostFrameCallback(
        (_) => _navigateToScreen(initialMessage.data['screen'] as String?),
      );
    }
  }

  Future<void> _initWeb() async {
    const vapidKey = String.fromEnvironment('FCM_VAPID_KEY');
    if (vapidKey.isEmpty) {
      debugPrint('[FCM] Web: FCM_VAPID_KEY not set — web push disabled');
      return;
    }

    final settings = await _messaging.requestPermission(
      alert: true,
      badge: true,
      sound: true,
    );

    if (settings.authorizationStatus == AuthorizationStatus.denied) {
      debugPrint('[FCM] Web: notification permission denied');
      return;
    }

    final token = await _messaging.getToken(vapidKey: vapidKey);
    if (token != null) {
      await _registerToken(token);
    }

    _messaging.onTokenRefresh.listen(_registerToken);

    // Browsers suppress the service-worker notification when the page is focused,
    // so handle foreground messages explicitly via the Notification API.
    FirebaseMessaging.onMessage.listen((message) {
      final title = message.notification?.title ??
          message.data['title'] as String? ??
          'Notification';
      final body = message.notification?.body ??
          message.data['body'] as String? ??
          '';
      showWebForegroundNotification(title, body);
      onMessageReceived?.call();
    });
  }

  /// Returns whether OS-level notifications are currently allowed.
  ///
  /// On Android 13+ this maps to the POST_NOTIFICATIONS runtime grant; on
  /// iOS/web it reflects the system-level authorization status. Returns
  /// `null` when the platform cannot answer (e.g. desktop) so callers can
  /// skip prompting.
  Future<bool?> areNotificationsEnabled() async {
    try {
      if (kIsWeb) {
        final settings = await _messaging.getNotificationSettings();
        return settings.authorizationStatus == AuthorizationStatus.authorized ||
            settings.authorizationStatus == AuthorizationStatus.provisional;
      }
      if (Platform.isAndroid) {
        return await _localNotifications
            .resolvePlatformSpecificImplementation<
                AndroidFlutterLocalNotificationsPlugin>()
            ?.areNotificationsEnabled();
      }
      if (Platform.isIOS) {
        final settings = await _messaging.getNotificationSettings();
        return settings.authorizationStatus == AuthorizationStatus.authorized ||
            settings.authorizationStatus == AuthorizationStatus.provisional;
      }
    } catch (e) {
      debugPrint('[FCM] areNotificationsEnabled failed: $e');
    }
    return null;
  }

  /// Re-requests notification permission. On Android the system prompt only
  /// appears the first time; once denied permanently the user must change it
  /// from system settings (use [openNotificationSettings]).
  Future<bool> requestPermission() async {
    try {
      if (kIsWeb || Platform.isIOS) {
        final settings = await _messaging.requestPermission(
          alert: true,
          badge: true,
          sound: true,
        );
        return settings.authorizationStatus == AuthorizationStatus.authorized ||
            settings.authorizationStatus == AuthorizationStatus.provisional;
      }
      if (Platform.isAndroid) {
        final granted = await _localNotifications
            .resolvePlatformSpecificImplementation<
                AndroidFlutterLocalNotificationsPlugin>()
            ?.requestNotificationsPermission();
        return granted ?? false;
      }
    } catch (e) {
      debugPrint('[FCM] requestPermission failed: $e');
    }
    return false;
  }

  /// Opens the OS settings page where the user can flip the notifications
  /// toggle for this app.
  Future<void> openNotificationSettings() async {
    try {
      await AppSettings.openAppSettings(type: AppSettingsType.notification);
    } catch (e) {
      debugPrint('[FCM] openNotificationSettings failed: $e');
    }
  }

  Future<void> _registerToken(String token) async {
    try {
      final platform = kIsWeb ? 'web' : (Platform.isIOS ? 'ios' : 'android');
      await DioClient().dio.post(
        ApiEndpoints.deviceToken,
        data: {'token': token, 'platform': platform},
      );
      debugPrint('[FCM] Device token registered ($platform)');
    } catch (e) {
      debugPrint('[FCM] Failed to register device token: $e');
    }
  }
}
