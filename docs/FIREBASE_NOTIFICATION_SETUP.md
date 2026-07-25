# Firebase Push Notification Setup — End to End

> **Project:** StitchCraftLearn (Design Express)
> **Stack:** Flutter app + Go backend + Docker on example.com
> **Android package:** `com.example.stitch_craft_learn`
> **iOS bundle:** `com.example.stitchcraftlearn`

---

## Overview

```
Firebase project "Design Express"
├── Android app  →  google-services.json        →  app/android/app/
├── iOS app      →  GoogleService-Info.plist    →  app/ios/Runner/
└── Service account  →  firebase-service-account.json  →  api/  (live server only)
```

All three share the same Firebase project so one service account covers all platforms.

---

## PART 1 — Firebase Console Setup

### Step 1 — Create Firebase project

1. Go to [https://console.firebase.google.com](https://console.firebase.google.com)
2. Click **"Add project"**
3. Project name: `Design Express`
4. Click **Continue**
5. Google Analytics: enable or disable (does not affect notifications)
6. Click **"Create project"** → wait for it to finish → click **Continue**

---

### Step 2 — Add Android app

1. On the project overview page, click the **Android icon** ( `</>` → Android)
2. Fill in:
   - **Android package name:** `com.example.stitch_craft_learn`
   - App nickname: `StitchCraft Android` (optional)
   - Debug signing certificate SHA-1: skip for now
3. Click **"Register app"**
4. Click **"Download google-services.json"**
5. Save this file — you will place it in the repo in Part 2
6. Click **Next** → **Next** → **Continue to console** (skip the SDK steps)

---

### Step 3 — Add iOS app

1. On the project overview page, click **"Add app"** → select **iOS icon**
2. Fill in:
   - **iOS bundle ID:** `com.example.stitchcraftlearn`
   - App nickname: `StitchCraft iOS` (optional)
3. Click **"Register app"**
4. Click **"Download GoogleService-Info.plist"**
5. Save this file — you will place it in the repo in Part 2
6. Click **Next** → **Next** → **Next** → **Continue to console** (skip the SDK steps)

---

### Step 4 — Enable FCM V1 API

1. In the project, click **gear ⚙️ (Project Settings)** → **Cloud Messaging** tab
2. Find **"Firebase Cloud Messaging API (V1)"**
3. If it shows a **"Enable"** button → click it
4. It should show: **Enabled ✅**

---

### Step 5 — Download Service Account (for backend)

1. Click **gear ⚙️ (Project Settings)** → **Service accounts** tab
2. Click **"Generate new private key"** button
3. Click **"Generate key"** in the popup
4. A JSON file downloads automatically
5. **Rename it to:** `firebase-service-account.json`
6. Keep this file safe — do NOT commit it to git

---

### Step 6 — iOS APNs Key (required for iOS notifications)

> Skip this if you only support Android for now.

1. Go to [Apple Developer Portal](https://developer.apple.com) → **Certificates, IDs & Profiles** → **Keys**
2. Click **+** → name it `FCM APNs Key` → check **Apple Push Notifications service (APNs)**
3. Click **Continue** → **Register** → **Download** the `.p8` file
4. Note the **Key ID** and your **Team ID** (shown in top right of Apple Developer portal)
5. Back in Firebase Console → **Project Settings** → **Cloud Messaging** → **Apple app configuration**
6. Click **"Upload"** under APNs Authentication Key
7. Upload the `.p8` file, enter Key ID and Team ID → **Upload**

---

## PART 2 — Local Code Changes

### Step 7 — Place config files in repo

```
# Place the files you downloaded:
app/android/app/google-services.json        ← from Step 2
app/ios/Runner/GoogleService-Info.plist     ← from Step 3
```

Both files are already in `.gitignore` — they will NOT be committed.

---

### Step 8 — Update firebase_options.dart

Open `app/lib/firebase_options.dart` and replace ALL content with the values
from your new Firebase project. Get the values from:

```
Firebase Console → Design Express → ⚙️ Project Settings → General tab → Your apps
```

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
    apiKey: String.fromEnvironment('FIREBASE_WEB_API_KEY'),
    appId: String.fromEnvironment('FIREBASE_WEB_APP_ID'),
    messagingSenderId: 'YOUR_SENDER_ID',        // ← from Firebase project settings
    projectId: 'YOUR_PROJECT_ID',               // ← your new project ID (may have suffix like your-project-xxxx)
    storageBucket: 'YOUR_STORAGE_BUCKET',
    authDomain: 'YOUR_AUTH_DOMAIN',
  );

  static const FirebaseOptions android = FirebaseOptions(
    apiKey: 'YOUR_ANDROID_API_KEY',             // ← from google-services.json → api_key → current_key
    appId: 'YOUR_ANDROID_APP_ID',               // ← from google-services.json → mobilesdk_app_id
    messagingSenderId: 'YOUR_SENDER_ID',        // ← from google-services.json → project_number
    projectId: 'YOUR_PROJECT_ID',               // ← from google-services.json → project_id
    storageBucket: 'YOUR_STORAGE_BUCKET',
  );

  static const FirebaseOptions ios = FirebaseOptions(
    apiKey: 'YOUR_IOS_API_KEY',                 // ← from GoogleService-Info.plist → API_KEY
    appId: 'YOUR_IOS_APP_ID',                   // ← from GoogleService-Info.plist → GOOGLE_APP_ID
    messagingSenderId: 'YOUR_SENDER_ID',        // ← from GoogleService-Info.plist → GCM_SENDER_ID
    projectId: 'YOUR_PROJECT_ID',               // ← from GoogleService-Info.plist → PROJECT_ID
    storageBucket: 'YOUR_STORAGE_BUCKET',
    iosBundleId: 'com.example.stitchcraftlearn',
  );
}
```

> **How to find values in google-services.json:**
> ```json
> {
>   "project_info": {
>     "project_number": "← this is messagingSenderId",
>     "project_id":     "← this is projectId"
>   },
>   "client": [{
>     "client_info": {
>       "mobilesdk_app_id": "← this is appId"
>     },
>     "api_key": [{ "current_key": "← this is apiKey" }]
>   }]
> }
> ```

---

### Step 9 — Commit and push

```bash
git add app/lib/firebase_options.dart
git commit -m "chore: update Firebase config to Design Express project"
git push origin main
```

Do NOT add `google-services.json` or `GoogleService-Info.plist` to git.

---

### Step 10 — Build APK

```bash
./scripts/build-apk.sh
```

This builds `app/build/app/outputs/apk/release/app-release.apk` with the live
server URL (`https://example.com/api/v1`) baked in.

---

## PART 3 — Live Server Setup

### Step 11 — Upload service account to server

Upload the `firebase-service-account.json` you downloaded in Step 5:

**Option A — from your local machine:**
```bash
scp firebase-service-account.json YOUR_USER@example.com:/path/to/repo/api/firebase-service-account.json
```

**Option B — from Google Cloud Shell:**
1. Click **⋮ menu** in Cloud Shell → **Upload file**
2. Select `firebase-service-account.json` from your computer (it uploads to `~/`)
3. Then copy it to the repo:
```bash
scp ~/firebase-service-account.json YOUR_USER@example.com:/path/to/repo/api/firebase-service-account.json
```

**Option C — paste directly on the server:**
```bash
ssh YOUR_USER@example.com
cd /path/to/repo
nano api/firebase-service-account.json
# paste the full JSON content → Ctrl+X → Y → Enter
```

---

### Step 12 — Redeploy the server

```bash
ssh YOUR_USER@example.com
cd /path/to/repo
git pull origin main
./scripts/deploy.sh
```

---

### Step 13 — Verify notifications are working

```bash
# Check that FCM initialized successfully
docker compose logs api | grep fcm

# You should see:
# fcm: initialized  project=your-firebase-project
#
# If you see this instead, the file is missing or wrong path:
# fcm: credentials file not found — push notifications disabled
```

---

### Step 14 — Verify device tokens are registering

After a user logs in on the new APK, check the database:

```bash
docker compose exec postgres psql -U admin -d marketkit -c "SELECT user_id, platform, updated_at FROM device_tokens ORDER BY updated_at DESC LIMIT 10;"
```

You should see rows with `platform = android` or `platform = ios`.

---

### Step 15 — Send a test notification

From the admin panel at `https://example.com`:
1. Go to **Notifications** page
2. Enter a title and message
3. Click **Send**
4. The Flutter app should receive it within a few seconds

Or via curl:
```bash
curl -X POST https://example.com/api/v1/admin/notifications/broadcast \
  -H "Authorization: Bearer YOUR_ADMIN_JWT" \
  -H "Content-Type: application/json" \
  -d '{"title": "Test", "message": "Hello from admin"}'
```

---

## Quick Reference — File Locations

| File | Location | Source |
|------|----------|--------|
| `google-services.json` | `app/android/app/` | Firebase Console → Android app |
| `GoogleService-Info.plist` | `app/ios/Runner/` | Firebase Console → iOS app |
| `firebase-service-account.json` | `api/` on live server only | Firebase Console → Service accounts |
| `firebase_options.dart` | `app/lib/` | Updated manually from Firebase values |

---

## Troubleshooting

| Problem | Check |
|---------|-------|
| `fcm: credentials file not found` | `firebase-service-account.json` missing from `api/` on server |
| `fcm: initialized` but no notification received | Check `device_tokens` table — may be empty (user needs to re-login on new APK) |
| Android notification not received | Verify `google-services.json` matches the same project as service account |
| iOS notification not received | APNs key not uploaded in Firebase Console (Step 6) |
| Token count is 0 in DB | APK was built pointing to wrong API URL — rebuild with `./scripts/build-apk.sh` |
