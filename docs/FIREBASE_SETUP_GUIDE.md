# Complete Firebase Setup Guide (Step-by-Step)

This guide will walk you through setting up Firebase from scratch for your Design Express app.

---

## PART 1: Firebase Console Setup

### Step 1.1 — Go to Firebase Console

1. Open browser and go to: https://console.firebase.google.com
2. Click **"Add project"** (or **"Create project"**)
3. Fill in:
   - **Project name:** `Design Express` (or your preferred name)
   - Click **Continue**
4. **Google Analytics:** Choose Yes or No (doesn't affect notifications)
5. Click **"Create project"** and wait 1-2 minutes

---

### Step 1.2 — Add Android App

After project is created, you should see the project dashboard.

1. Look for Android icon (or click **"Add app"** → select Android)
2. Fill in:
   - **Android package name:** `com.example.stitch_craft_learn`
   - **App nickname (optional):** `StitchCraft Android`
3. Click **"Register app"**
4. Next screen: **Download google-services.json**
   - Click the download button
   - **Save this file** — you'll place it in `app/android/app/` later
5. Skip the next SDK setup screens: Click **Next** → **Next** → **Continue to console**

---

### Step 1.3 — Add iOS App

1. Back on project dashboard, click **"Add app"** → select iOS icon
2. Fill in:
   - **iOS bundle ID:** `com.example.stitchcraftlearn`
   - **App nickname (optional):** `StitchCraft iOS`
3. Click **"Register app"**
4. Next screen: **Download GoogleService-Info.plist**
   - Click the download button
   - **Save this file** — you'll place it in `app/ios/Runner/` later
5. Skip the next SDK setup screens: Click **Next** → **Next** → **Next** → **Continue to console**

---

### Step 1.4 — Enable FCM V1 API

1. In Firebase Console, click **⚙️ (gear icon)** → **Project Settings**
2. Click the **"Cloud Messaging"** tab
3. Look for **"Firebase Cloud Messaging API (V1)"**
4. If it shows **"Enable"** button → click it
5. Wait a few seconds, it should show **"Enabled ✅"**

---

### Step 1.5 — Download Service Account (for backend server)

1. Still in **Project Settings**, click the **"Service accounts"** tab
2. Click **"Generate new private key"** button
3. A popup appears: click **"Generate key"**
4. A **JSON file downloads automatically** (name: `[project-id]-firebase-adminsdk-[random].json`)
5. **Rename it to:** `firebase-service-account.json`
6. **Keep this file safe** — do NOT commit to git, upload to server only

---

### Step 1.6 — iOS APNs Key (required for iOS notifications)

> **Note:** If you only support Android for now, you can skip this step and come back to it later.

1. Go to [Apple Developer Portal](https://developer.apple.com/account) and log in
2. Go to **Certificates, IDs & Profiles** → **Keys**
3. Click **"+" button** to create a new key
4. Fill in:
   - **Key name:** `FCM APNs Key`
   - Check **"Apple Push Notifications service (APNs)"**
5. Click **Continue** → **Register** → **Download** the `.p8` file
6. Note down your **Key ID** (shown on the page) and your Apple **Team ID** (top right corner)
7. Back in Firebase Console:
   - **Project Settings** → **Cloud Messaging** → **Apple app configuration**
   - Click **"Upload"** under APNs Authentication Key
   - Upload the `.p8` file
   - Enter **Key ID** and **Team ID** → click **Upload**

---

## PART 2: Code Changes (Flutter App)

### Step 2.1 — Restore Firebase Packages to pubspec.yaml

Open `app/pubspec.yaml` and add the Firebase packages back.

Find this section:
```yaml
  # Push notifications (Firebase removed — to be re-added)
  flutter_local_notifications: ^17.2.2
  app_settings: ^5.1.1
```

Replace it with:
```yaml
  # Push notifications
  firebase_core: ^3.6.0
  firebase_messaging: ^15.1.3
  flutter_local_notifications: ^17.2.2
  app_settings: ^5.1.1
```

Save the file.

---

### Step 2.2 — Place Google Services Files

1. **For Android:**
   - Download file: `google-services.json` (from Step 1.2)
   - Place it at: `app/android/app/google-services.json`
   - This file is already in `.gitignore` — it won't be committed

2. **For iOS:**
   - Download file: `GoogleService-Info.plist` (from Step 1.3)
   - Place it at: `app/ios/Runner/GoogleService-Info.plist`
   - This file is already in `.gitignore` — it won't be committed

---

### Step 2.3 — Update firebase_options.dart

1. Open `app/lib/firebase_options.dart` and replace the entire content:

```dart
import 'package:firebase_core/firebase_core.dart' show FirebaseOptions;
import 'package:flutter/foundation.dart'
    show defaultTargetPlatform, kIsWeb, TargetPlatform;

class DefaultFirebaseOptions {
  static FirebaseOptions get currentPlatform {
    if (kIsWeb) {
      return web;
    }
    switch (defaultTargetPlatform) {
      case TargetPlatform.android:
        return android;
      case TargetPlatform.iOS:
        return ios;
      default:
        throw UnsupportedError(
          'DefaultFirebaseOptions are not supported for this platform.',
        );
    }
  }

  static const FirebaseOptions web = FirebaseOptions(
    apiKey: 'YOUR_FIREBASE_WEB_API_KEY',
    appId: 'YOUR_FIREBASE_WEB_APP_ID',
    messagingSenderId: 'YOUR_SENDER_ID',
    projectId: 'YOUR_PROJECT_ID',
    storageBucket: 'YOUR_STORAGE_BUCKET',
    authDomain: 'YOUR_AUTH_DOMAIN',
  );

  static const FirebaseOptions android = FirebaseOptions(
    apiKey: 'YOUR_ANDROID_API_KEY',
    appId: 'YOUR_ANDROID_APP_ID',
    messagingSenderId: 'YOUR_SENDER_ID',
    projectId: 'YOUR_PROJECT_ID',
    storageBucket: 'YOUR_STORAGE_BUCKET',
  );

  static const FirebaseOptions ios = FirebaseOptions(
    apiKey: 'YOUR_IOS_API_KEY',
    appId: 'YOUR_IOS_APP_ID',
    messagingSenderId: 'YOUR_SENDER_ID',
    projectId: 'YOUR_PROJECT_ID',
    storageBucket: 'YOUR_STORAGE_BUCKET',
    iosClientId: null,
    iosBundleId: 'com.example.stitchcraftlearn',
  );
}
```

---

### Step 2.4 — Get Firebase Values

You need to fill in the placeholders in `firebase_options.dart` with real values.

**To get these values:**

1. Go to Firebase Console → Your Project → **⚙️ Project Settings**
2. Click **"General"** tab
3. Scroll down to **"Your apps"** section
4. Find your **Android** app → click it

**From google-services.json (Android values):**
- Open the `google-services.json` file you downloaded
- Look for these values:
  - `messagingSenderId` → use for `messagingSenderId`
  - `project_id` → use for `projectId`
  - Inside `client` array, find `client_info.mobilesdk_app_id` → use for `appId`
  - Inside `client_info`, find `api_key[0].current_key` → use for `apiKey`

**For iOS and Web:**
- Do the same but use values from `GoogleService-Info.plist` (for iOS)
- For Web, use values from Firebase Console → Project Settings → Your Web app

**Example filled-in file:**
```dart
  static const FirebaseOptions android = FirebaseOptions(
    apiKey: 'YOUR_ANDROID_API_KEY',
    appId: 'YOUR_ANDROID_APP_ID',
    messagingSenderId: 'YOUR_SENDER_ID',
    projectId: 'YOUR_PROJECT_ID',
    storageBucket: 'YOUR_STORAGE_BUCKET',
  );
```

---

### Step 2.5 — Restore Firebase Code in main.dart

Open `app/lib/main.dart` and replace the beginning:

```dart
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
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

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await Firebase.initializeApp(options: DefaultFirebaseOptions.currentPlatform);
  if (!kIsWeb) {
    FirebaseMessaging.onBackgroundMessage(_backgroundMessageHandler);
  }
```

---

### Step 2.6 — Restore Firebase Code in notification_service.dart

Open `app/lib/core/services/notification_service.dart` and restore the Firebase import at the top:

Find this:
```dart
import 'dart:io' show Platform;
import 'dart:typed_data';

import 'package:app_settings/app_settings.dart';
import 'package:dio/dio.dart';
// Firebase messaging removed — to be re-added during fresh setup
```

Replace with:
```dart
import 'dart:io' show Platform;
import 'dart:typed_data';

import 'package:app_settings/app_settings.dart';
import 'package:dio/dio.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
```

And restore the `_messaging` field in the `NotificationService` class:

Find:
```dart
  // Firebase messaging instance removed — to be re-added during fresh setup
```

Replace with:
```dart
  final _messaging = FirebaseMessaging.instance;
```

---

### Step 2.7 — Restore Firebase Code in auth_service.dart

Open `app/lib/features/auth/services/auth_service.dart`

Find these commented lines in `verifyOtp()` method:
```dart
    // Firebase notification setup removed — to be re-added during fresh setup
    // if (!kIsWeb) {
    //   unawaited(NotificationService().init());
    // }
```

Uncomment them:
```dart
    if (!kIsWeb) {
      unawaited(NotificationService().init());
    }
```

Do the same in `tryRefresh()` method.

---

### Step 2.8 — Restore Web Firebase Service Worker

Open `app/web/firebase-messaging-sw.js` and replace entire content with:

```javascript
importScripts('https://www.gstatic.com/firebasejs/10.14.0/firebase-app-compat.js');
importScripts('https://www.gstatic.com/firebasejs/10.14.0/firebase-messaging-compat.js');

// Load FIREBASE_WEB_API_KEY and FIREBASE_WEB_APP_ID into the SW global scope.
// firebase-config.js is gitignored and generated at build/deploy time from
// environment variables. See web/firebase-config.js.template for the format.
try {
  importScripts('/firebase-config.js');
} catch (_) {
  // Running without firebase-config.js (local dev without the file, or first
  // deploy before it was generated). Background push will be silently disabled.
  console.warn('[FCM SW] /firebase-config.js not found — background notifications disabled. See web/firebase-config.js.template.');
}

if (self.FIREBASE_WEB_API_KEY && self.FIREBASE_WEB_APP_ID) {
  firebase.initializeApp({
    apiKey: self.FIREBASE_WEB_API_KEY,
    authDomain: 'YOUR_AUTH_DOMAIN',
    projectId: 'YOUR_PROJECT_ID',
    storageBucket: 'YOUR_STORAGE_BUCKET',
    messagingSenderId: 'YOUR_SENDER_ID',
    appId: self.FIREBASE_WEB_APP_ID,
  });

  firebase.messaging().onBackgroundMessage((payload) => {
    const title = payload.notification?.title ?? payload.data?.title ?? 'Notification';
    const body  = payload.notification?.body  ?? payload.data?.body  ?? '';
    self.registration.showNotification(title, {
      body,
      icon: '/icons/Icon-192.png',
      data: { screen: payload.data?.screen ?? '' },
    });
  });
}

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const screen = event.notification.data?.screen ?? '';
  const pathMap = { gallery: '/gallery', community: '/community', home: '/home' };
  const path = pathMap[screen] ?? '/home';
  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windowClients) => {
      for (const client of windowClients) {
        if (client.url.includes(self.location.origin) && 'focus' in client) {
          client.navigate(path);
          return client.focus();
        }
      }
      return clients.openWindow(path);
    }),
  );
});
```

Replace `YOUR_*` placeholders with your Firebase values.

---

### Step 2.9 — Update Flutter Dependencies

From `app/` directory:
```bash
cd app
flutter pub get
flutter clean
```

---

## PART 3: Server Setup

### Step 3.1 — Place Service Account on Server

1. You have `firebase-service-account.json` (from Step 1.5)
2. Upload to server at: `api/secrets/firebase-service-account.json`

**From your local machine:**
```bash
scp firebase-service-account.json YOUR_USER@example.com:/home/YOUR_USER/marketkit/api/secrets/firebase-service-account.json
```

Or **on the server directly:**
```bash
# SSH into server
ssh youruser@example.com

# Or from repo on server
cd ~/marketkit
nano api/secrets/firebase-service-account.json
# Paste entire JSON content → Ctrl+X → Y → Enter
```

---

### Step 3.2 — Verify File is Readable

```bash
ls -l api/secrets/firebase-service-account.json
jq -r .project_id api/secrets/firebase-service-account.json
```

Should output your Firebase project ID.

---

### Step 3.3 — Rebuild and Deploy Backend

From repo root:
```bash
docker compose build api
docker compose up -d
```

Wait 10-15 seconds for container to start.

---

### Step 3.4 — Verify FCM Initialized

```bash
docker compose logs api | grep fcm
```

**Expected output:**
```
fcm: initialized  project=your-firebase-project
```

**If you see this instead, the file is missing:**
```
fcm: credentials file not found — push notifications disabled
```

---

## PART 4: Build and Deploy App

### Step 4.1 — Rebuild APK with Correct Backend URL

From repo root:
```bash
API_BASE_URL=https://example.com/api/v1 ./scripts/build-apk.sh
```

This creates: `app/build/app/outputs/apk/release/app-release.apk`

---

### Step 4.2 — Install APK on Device

Transfer APK to your Android device and install.

---

### Step 4.3 — Test Notification Registration

1. Open app
2. **Register** a new account or **Login**
3. **Allow notifications** when prompted
4. Open app settings → allow POST_NOTIFICATIONS (Android 13+)

---

### Step 4.4 — Verify Token Registered

On server:
```bash
docker compose exec postgres psql -U admin -d marketkit -c "SELECT user_id, platform, updated_at FROM device_tokens ORDER BY updated_at DESC LIMIT 10;"
```

**Expected output:**
```
         user_id          | platform | updated_at
---------+----------+------------
 user-123 | android  | 2026-07-03 10:30:15.123456
```

If count is 0, check:
- App debug logs for `[FCM] Device token registered`
- Ensure backend URL in app is correct
- User is logged in before notifications are allowed

---

### Step 4.5 — Check Server Logs for Token Registration

```bash
docker compose logs api | grep "device token registration"
```

Should show:
```
device token registration user_id=user-123 platform=android token_prefix=dxyz...
```

---

## PART 5: Test End-to-End

### Step 5.1 — Send Test Broadcast

**Via Admin UI:**
1. Open https://example.com
2. Login as admin
3. Go to **Notifications** page
4. Enter title and message
5. Click **Send**

**Via curl:**
```bash
curl -X POST https://example.com/api/v1/admin/notifications/broadcast \
  -H "Authorization: Bearer YOUR_ADMIN_JWT" \
  -H "Content-Type: application/json" \
  -d '{"title":"Test","message":"Hello from admin"}'
```

---

### Step 5.2 — Verify Device Receives Notification

1. Make sure app is installed and user is logged in
2. Send broadcast (from Step 5.1)
3. Device should receive push notification within 2-3 seconds
4. Tap notification to open app

---

### Step 5.3 — Check Server Logs

```bash
docker compose logs api | tail -n 50 | grep fcm
```

Should show messages like:
```
fcm: send  user_id=user-123 (sent/failed counts)
```

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `fcm: credentials file not found` | Upload `firebase-service-account.json` to `api/secrets/` on server |
| App doesn't register token | Check debug logs for `[FCM] Device token registered` or errors |
| Tokens empty in DB | Rebuild APK with correct `API_BASE_URL`, reinstall, re-login |
| Token registered but no notification | Check Firebase project IDs match between app and server |
| Android notification not received | Verify `google-services.json` is in `app/android/app/` |
| iOS notification not received | Verify APNs key uploaded in Firebase Console, `GoogleService-Info.plist` in place |

---

## Next Steps

After successful setup:
1. Commit `app/lib/firebase_options.dart` to git
2. DO NOT commit `.json` or `.plist` files (already in `.gitignore`)
3. Push to `notification` branch for testing
4. Merge to `main` once verified

Good luck! 🚀
