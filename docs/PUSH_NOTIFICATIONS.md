# Push Notifications — Reference Guide

> **Project:** StitchCraftLearn  
> **Stack:** Go (Fiber) API + Flutter App  
> **Service:** Firebase Cloud Messaging (FCM) V1 API  
> **Last updated:** April 2026

---

## Table of Contents

1. [How It Works](#how-it-works)
2. [Architecture Overview](#architecture-overview)
3. [Trigger Points](#trigger-points)
4. [File Reference](#file-reference)
5. [Installation & Setup](#installation--setup)
6. [Environment Variables](#environment-variables)
7. [Testing](#testing)
8. [Troubleshooting](#troubleshooting)

---

## How It Works

```
Admin publishes content
        ↓
Go API handler detects publish event
        ↓
fcm.SendToAll(title, body) called in goroutine
        ↓
API reads all device tokens from DB (device_tokens table)
        ↓
Authenticates with Firebase using service account JSON (OAuth2)
        ↓
Sends one FCM V1 HTTP request per device token
        ↓
FCM delivers notification to device
        ↓
Flutter app receives it:
  - Foreground → flutter_local_notifications shows it as a banner
  - Background → FCM auto-displays system notification
  - Terminated → FCM auto-displays system notification
```

---

## Architecture Overview

### Backend (Go)

- **FCM package:** `api/internal/fcm/fcm.go`
- Uses **FCM HTTP V1 API** (not the deprecated legacy API)
- Authenticates via **OAuth2 service account** (not a server key)
- Endpoint: `https://fcm.googleapis.com/v1/projects/stitchcraftlearn/messages:send`
- Credentials loaded lazily on first use from `firebase-service-account.json`
- FCM calls run in a **goroutine** (`go fcm.SendToAll(...)`) so they never block the API response

### Flutter App

- **Firebase packages:** `firebase_core`, `firebase_messaging`
- **Local notifications:** `flutter_local_notifications` (for foreground display)
- **Token registration:** On login AND on every app refresh (`tryRefresh`)
- **Notification service:** `app/lib/core/services/notification_service.dart`

### Device Token Storage

Table: `device_tokens`

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| user_id | string | Links to users table |
| token | string | FCM registration token (unique) |
| platform | string | `android` or `ios` |
| updated_at | timestamp | Last registration time |

One user can have multiple tokens (multiple devices).

---

## Trigger Points

Notifications are automatically sent when admin performs these actions:

### 1. Video Published
- **Where:** `api/internal/modules/videos/handler.go` → `HandleUpdate`
- **Condition:** `status` field changes to `PUBLISHED` (from any other status)
- **Title:** `New Video Published`
- **Body:** Video title

### 2. Photo Published
- **Where:** `api/internal/modules/photos/handler.go` → `HandleUpdate`
- **Condition:** `is_published` field changes from `false` → `true`
- **Title:** `New Photos Added`
- **Body:** Photo title

### 3. Admin Community Post
- **Where:** `api/internal/modules/community/admin_handler.go` → `HandleAdminCreatePost`
- **Endpoint:** `POST /api/v1/community/posts` (requires admin JWT)
- **Condition:** Every time admin creates a community post
- **Title:** `New Community Post`
- **Body:** Post title

---

## File Reference

### Backend

| File | Purpose |
|------|---------|
| `api/internal/fcm/fcm.go` | FCM V1 client — `SendToAll()` and `SendToUser()` |
| `api/internal/models/device_token.go` | DeviceToken DB model |
| `api/internal/modules/videos/handler.go` | Triggers notification on video publish |
| `api/internal/modules/photos/handler.go` | Triggers notification on photo publish |
| `api/internal/modules/community/admin_handler.go` | Admin post creation + notification trigger |
| `api/internal/modules/community/routes.go` | Registers `POST /community/posts` admin route |
| `api/internal/modules/notifications/handler.go` | Manual broadcast endpoint (admin panel) |
| `api/firebase-service-account.json` | Firebase service account credentials (**never commit this**) |

### Flutter App

| File | Purpose |
|------|---------|
| `app/lib/main.dart` | Firebase init + background handler registration |
| `app/lib/firebase_options.dart` | Firebase project config (API keys, app IDs) |
| `app/lib/core/services/notification_service.dart` | Permission request, token registration, message handling |
| `app/lib/features/auth/services/auth_service.dart` | Calls `NotificationService().init()` on login and refresh |
| `app/lib/core/network/api_endpoints.dart` | Device token endpoint: `/user/auth/device-token` |
| `app/android/app/google-services.json` | Firebase config for Android (**never commit this**) |
| `app/ios/Runner/GoogleService-Info.plist` | Firebase config for iOS (**never commit this**) |

---

## Installation & Setup

### Prerequisites

- Firebase project created at [console.firebase.google.com](https://console.firebase.google.com)
- Both Android and iOS apps registered in the Firebase project
- FCM V1 API enabled (check: Project Settings → Cloud Messaging → Firebase Cloud Messaging API (V1) = Enabled)

### Step 1: Backend — Service Account

1. Firebase Console → gear ⚙️ → **Project settings** → **Service accounts** tab
2. Click **"Generate new private key"** → **"Generate key"**
3. Rename the downloaded file to `firebase-service-account.json`
4. Place it at: `api/firebase-service-account.json`

> The file is read automatically. No environment variable needed unless you want a custom path.

### Step 1b: Docker deployment — mount the credentials file

The production Docker container does **not** bundle the service account file (it is git-ignored). You must place the file on the server and it is already mounted via `docker-compose.yml`:

```yaml
# docker-compose.yml — api service volumes (already configured):
volumes:
  - ./api/uploads:/app/uploads
  - ./api/firebase-service-account.json:/app/firebase-service-account.json:ro
```

**On the live server, before running `./scripts/deploy.sh`:**

```bash
# Place the service account file on the server
scp firebase-service-account.json user@example.com:/path/to/repo/api/firebase-service-account.json

# Then deploy
./scripts/deploy.sh
```

Verify it is working after deploy:
```bash
docker compose logs api | grep fcm
# Should show: fcm: initialized  project=<your-project-id>
# NOT: fcm: credentials file not found — push notifications disabled
```

> **Important — register both platforms under ONE Firebase project.** The service
> account you upload must belong to the same Firebase project that issued the device
> tokens stored in your database. If your Android and iOS apps are registered under
> two different projects, only one platform will receive notifications — a single
> service account cannot send to both. Add both apps to one project before going live.

### Step 2: Flutter Android — google-services.json

1. Firebase Console → Project settings → **General** tab
2. Under "Your apps", select the Android app (`com.example.stitch_craft_learn`)
3. Click **"Download google-services.json"**
4. Place it at: `app/android/app/google-services.json`

### Step 3: Flutter iOS — GoogleService-Info.plist

1. Firebase Console → Project settings → **General** tab
2. Under "Your apps", select the iOS app (`com.example.stitchcraftlearn`)
3. Click **"Download GoogleService-Info.plist"**
4. Place it at: `app/ios/Runner/GoogleService-Info.plist`

> **iOS extra step:** Push Notifications capability must be enabled in Xcode:
> Xcode → Runner target → Signing & Capabilities → + Capability → Push Notifications

### Step 4: Install Flutter dependencies

```bash
cd app
flutter pub get
```

### Step 5: Run and test

```bash
# Terminal 1 — start API
cd api
go run ./cmd/api

# Terminal 2 — run Flutter on real device
cd app
flutter run
```

> Push notifications do **not** work on iOS simulators. Use a real device.  
> Android emulators work for FCM but a real device is more reliable.

---

## Environment Variables

Add to `api/.env`:

```env
# Optional — defaults to ./firebase-service-account.json
# Only set this if the file is in a different location
FIREBASE_CREDENTIALS_FILE=./firebase-service-account.json
```

`FCM_SERVER_KEY` is **no longer used** — it was for the deprecated legacy API.

---

## Testing

### Verify device token is registered

After logging in on a device, check the database:

```sql
SELECT * FROM device_tokens ORDER BY updated_at DESC LIMIT 10;
```

You should see a row with the user's ID and a long FCM token string.

### Trigger a notification manually

Using the admin broadcast endpoint:

```bash
curl -X POST http://localhost:3000/api/v1/admin/notifications/broadcast \
  -H "Authorization: Bearer <admin_jwt_token>" \
  -H "Content-Type: application/json" \
  -d '{"title": "Test", "message": "Hello from admin"}'
```

### Trigger via admin actions

| Action | API call |
|--------|----------|
| Publish a video | `PATCH /api/v1/videos/:id` with `{"status": "PUBLISHED"}` |
| Publish a photo | `PATCH /api/v1/photos/:id` with `{"is_published": true}` |
| Create community post | `POST /api/v1/community/posts` with `{"title":"...", "content":"..."}` |

### Expected behaviour

| App state | Behaviour |
|-----------|-----------|
| App open (foreground) | Local notification banner appears via `flutter_local_notifications` |
| App in background | System notification appears in notification tray |
| App closed (terminated) | System notification appears in notification tray |

---

## Troubleshooting

### Notification not received

**1. Check device token is in the database**
```sql
SELECT COUNT(*) FROM device_tokens;
```
If count is 0, the app never registered a token. Check that:
- User is logged in
- `NotificationService().init()` was called
- Firebase is properly initialized (`firebase_options.dart` is present)

**2. Check API logs**
Look for `[FCM]` log lines when a video/photo is published. If you see `push notifications disabled`, the `firebase-service-account.json` file is missing or unreadable.

**3. Check FCM credentials**
```bash
ls -la api/firebase-service-account.json
```
File must exist and be readable by the API process.

**4. Verify FCM V1 API is enabled**
Firebase Console → Project settings → Cloud Messaging → "Firebase Cloud Messaging API (V1)" must show ✅ Enabled.

---

### Foreground notifications not showing

The app uses `flutter_local_notifications` to show notifications when the app is open. Check:
- `flutter_local_notifications` is in `pubspec.yaml`
- `flutter pub get` was run after adding it
- On Android, the notification channel `default_channel` exists (created automatically on first `init()` call)

---

### Android build fails after Firebase setup

Ensure both Gradle files have the `google-services` plugin:

**`android/settings.gradle.kts`** must contain:
```kotlin
id("com.google.gms.google-services") version "4.4.2" apply false
```

**`android/app/build.gradle.kts`** must contain:
```kotlin
id("com.google.gms.google-services")
```

---

### iOS notifications not working

1. Push Notifications capability must be added in Xcode (Runner → Signing & Capabilities → Push Notifications)
2. `GoogleService-Info.plist` must be present in `ios/Runner/`
3. Must test on a **real device** — iOS simulator does not support push notifications
4. For production, an APNs authentication key must be uploaded:
   Firebase Console → Project settings → Cloud Messaging → Apple app configuration → Upload APNs key

---

### Token rotation / stale tokens

FCM tokens can change (app reinstall, OS update). The app handles this automatically:
- `_messaging.onTokenRefresh` listener re-registers the new token
- The backend upserts tokens — if a token already exists for another user, it reassigns to the current user

If users stop receiving notifications after a long time, it likely means their token rotated and the re-registration failed (e.g. network error). They can fix this by logging out and back in.

---

### FCM V1 API vs Legacy API

| | Legacy API (deprecated) | V1 API (current) |
|-|------------------------|-----------------|
| Status | Shut down June 2024 | Active |
| Auth | Server key string | OAuth2 service account |
| Batch send | Yes (`registration_ids` array) | No (one token per request) |
| Our implementation | ❌ Removed | ✅ In use |

---

## Security Notes

These files contain sensitive credentials and must **never be committed to git**:

- `api/firebase-service-account.json`
- `app/android/app/google-services.json`
- `app/ios/Runner/GoogleService-Info.plist`

They should be added to `.gitignore`. If accidentally committed, regenerate the service account key immediately from Firebase Console.
