import 'dart:async';

import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_crashlytics/firebase_crashlytics.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart' show debugPrint, kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'core/network/dio_client.dart';
import 'core/router/app_router.dart';
import 'core/theme/app_theme.dart';
import 'firebase_options.dart';
import 'shared/widgets/offline_banner.dart';

/// Must be a top-level function — firebase_messaging requires it.
@pragma('vm:entry-point')
Future<void> _backgroundMessageHandler(RemoteMessage message) async {
  // FCM auto-displays background notifications on Android/iOS.
  // No additional work needed unless custom data handling is required.
}

void main() {
  runZonedGuarded(() async {
    WidgetsFlutterBinding.ensureInitialized();
    try {
      await Firebase.initializeApp(options: DefaultFirebaseOptions.currentPlatform);
      if (!kIsWeb) {
        FirebaseMessaging.onBackgroundMessage(_backgroundMessageHandler);
        FlutterError.onError = FirebaseCrashlytics.instance.recordFlutterFatalError;
      }
    } catch (e) {
      debugPrint('[Firebase] init failed: $e');
    }

    // Force logout and show message when this account is opened on another device
    DioClient.onSessionReplaced = () {
      appRouter.go('/login');
      // Small delay so the route change settles before showing the snackbar
      Future.delayed(const Duration(milliseconds: 300), () {
        final context = appRouter.routerDelegate.navigatorKey.currentContext;
        if (context != null) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text(
                  'Your account was signed in on another device. Please log in again.'),
              duration: Duration(seconds: 5),
            ),
          );
        }
      });
    };

    runApp(const ProviderScope(child: StitchCraftLearnApp()));
  }, (error, stack) {
    debugPrint('[Uncaught] $error');
    if (!kIsWeb) {
      try {
        FirebaseCrashlytics.instance.recordError(error, stack, fatal: true);
      } catch (e) {
        debugPrint('[Crashlytics] recordError failed: $e');
      }
    }
  });
}

class StitchCraftLearnApp extends StatelessWidget {
  const StitchCraftLearnApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp.router(
      title: 'Stitch Craft Learn',
      theme: buildAppTheme(),
      routerConfig: appRouter,
      debugShowCheckedModeBanner: false,
      builder: (context, child) => OfflineBanner(child: child!),
    );
  }
}
