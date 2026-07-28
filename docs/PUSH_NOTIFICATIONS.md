# Push Notifications — Reference Guide

> **Project:** MarketKit  
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
- Endpoint: `https://fcm.googleapis.com/v1/projects/your-firebase-project/messages:send`
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
| `api/internal/routes.go` | Registers every route, including `POST /community/posts` |
| `api/internal/modules/notifications/handler.go` | Manual broadcast endpoint (admin panel) |
| `api/secrets/firebase-service-account.json` | Firebase service account credentials (**never commit this**) |

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
4. Place it at: `api/secrets/firebase-service-account.json`

> This exact path matters. `docker-compose.yml` mounts `./api/secrets` into the
> container and points `FIREBASE_CREDENTIALS_FILE` at it. A file left anywhere
> else is never read, and push notifications fail silently.

> The file is read automatically. No environment variable needed unless you want a custom path.

### Step 1b: Docker deployment — mount the credentials file

The production Docker container does **not** bundle the service account file (it is git-ignored). You must place the file on the server and it is already mounted via `docker-compose.yml`:

```yaml
# docker-compose.yml — api service volumes (already configured):
volumes:
  - ./api/uploads:/app/uploads
  - ./api/secrets:/app/secrets:ro
```

**On the live server, before running `./scripts/deploy.sh`:**

```bash
# Place the service account file on the server
scp firebase-service-account.json user@example.com:/path/to/repo/api/secrets/firebase-service-account.json

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
2. Under "Your apps", select the Android app (`com.example.marketkit`)
3. Click **"Download google-services.json"**
4. Place it at: `app/android/app/google-services.json`

### Step 3: Flutter iOS — GoogleService-Info.plist

1. Firebase Console → Project settings → **General** tab
2. Under "Your apps", select the iOS app (`com.example.marketkit`)
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
ls -la api/secrets/firebase-service-account.json
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

- `api/secrets/firebase-service-account.json`
- `app/android/app/google-services.json`
- `app/ios/Runner/GoogleService-Info.plist`

They should be added to `.gitignore`. If accidentally committed, regenerate the service account key immediately from Firebase Console.

---

# Troubleshooting

Merged from the former `NOTIFICATION_TROUBLESHOOTING.md`, so notification
problems and notification behaviour live in one place.

## ⚠️ Critical Issue Found: Missing Firebase Credentials

**Status:** ❌ **NOTIFICATIONS ARE CURRENTLY DISABLED**

### The Problem
- Firebase service account JSON file is **missing**
- Location: `/home/youruser/marketkit/api/secrets/firebase-service-account.json`
- Current state: Directory is empty (only `.gitkeep` file)
- Impact: FCM cannot authenticate, notifications cannot be sent

### Solution
You must place the Firebase service account credentials in the correct location.

**From FIREBASE_SETUP_GUIDE.md Step 1.5:**
```bash
# Download from Firebase Console → Project Settings → Service Accounts
# Then rename and upload to server:

scp firebase-service-account.json youruser@example.com:/home/youruser/marketkit/api/secrets/

# OR manually on the server:
nano /home/youruser/marketkit/api/secrets/firebase-service-account.json
# Paste the entire JSON content → Ctrl+X → Y → Enter
```

---

## 🔍 Step-by-Step Verification Process

### Step 1: Verify Firebase Credentials

```bash
# Check if credentials file exists
ls -la /home/youruser/marketkit/api/secrets/firebase-service-account.json

# Extract and verify project ID
jq -r '.project_id' /home/youruser/marketkit/api/secrets/firebase-service-account.json
# Expected: Your Firebase project ID (e.g., "your-firebase-project")
```

### Step 2: Check API Initialization (After Adding Credentials)

After placing credentials, restart the backend:

```bash
cd /home/youruser/marketkit
docker compose down
docker compose up -d
sleep 5
docker compose logs api | grep -i "fcm"
```

**Expected output:**
```
fcm: initialized  project=your-firebase-project
```

**If you see this instead:**
```
fcm: credentials file not found — push notifications disabled
```
→ The file is missing or in the wrong location

### Step 3: Check Device Tokens Are Registered

```bash
# Connect to database
docker compose exec postgres psql -U admin -d marketkit

# Inside psql, run:
SELECT user_id, platform, updated_at FROM device_tokens ORDER BY updated_at DESC LIMIT 10;
```

**Expected output:**
```
         user_id          | platform | updated_at
---------+----------+------------
 user-123 | android  | 2026-07-06 10:30:15.123456
 user-456 | ios      | 2026-07-06 10:25:30.654321
(2 rows)
```

**If empty:** Users haven't logged in and registered their device yet

### Step 4: Check Notification Logs

```bash
# Inside psql:
SELECT id, title, message, audience, sent_count, failed_count, created_at 
FROM notification_logs 
ORDER BY created_at DESC 
LIMIT 5;
```

**Expected output:**
```
 id | title | message | audience | sent_count | failed_count | created_at
 1  | Test  | Hello   | all      | 5          | 0            | 2026-07-06 11:00:00
```

### Step 5: Test Sending a Notification from Admin Panel

1. Open admin panel: https://example.com
2. Navigate to **Notifications** page
3. Fill in:
   - **Title:** "Test Notification"
   - **Message:** "This is a test"
   - **Audience:** "All Users"
4. Click **Send**
5. You should see success message

### Step 6: Verify Backend Processed the Notification

```bash
# Check API logs
docker compose logs api | grep -i "fcm\|broadcast\|send" | tail -20
```

**Expected log messages:**
```
fcm: send  user_id=user-123 (5 sent, 0 failed)
```

### Step 7: Check App Receives the Notification

On the mobile device:
1. App must be running or in background
2. Check debug console for: `[FCM] received message`
3. Should see notification on device screen
4. Tap it to open app

---

## 📋 Pre-Testing Checklist

- [ ] Firebase credentials file placed in `/api/secrets/firebase-service-account.json`
- [ ] Backend restarted and FCM initialized successfully
- [ ] At least 1 test user has registered their device token (logged in from mobile app)
- [ ] Notification permission is enabled on the mobile device
- [ ] Admin account has notification sending permissions

---

## 🧪 Full End-to-End Test

### Setup Phase (One time)

```bash
# 1. Place Firebase credentials
scp firebase-service-account.json youruser@example.com:/home/youruser/marketkit/api/secrets/

# 2. Restart backend
ssh youruser@example.com
cd ~/marketkit
docker compose down
docker compose up -d api
sleep 5

# 3. Verify FCM initialized
docker compose logs api | grep "fcm: initialized"
```

### Test Phase (Repeatable)

```bash
# 1. Register test device - Open mobile app, login as test user
# (This registers a device token)

# 2. Verify token registered in database
docker compose exec postgres psql -U admin -d marketkit -c "SELECT COUNT(*) FROM device_tokens;"

# 3. Send notification from admin panel
# (Open web browser, go to Admin → Notifications, send test message)

# 4. Check backend logs
docker compose logs api -f | grep -i "fcm"

# 5. Check mobile device receives notification
# (Look for push notification on screen)

# 6. Verify database recorded the notification
docker compose exec postgres psql -U admin -d marketkit -c "SELECT * FROM notification_logs ORDER BY created_at DESC LIMIT 1;"
```

---

## 🔧 Troubleshooting Common Issues

### Issue: "fcm: credentials file not found"

**Cause:** Firebase service account JSON not placed in the right location

**Fix:**
```bash
# 1. Verify file exists
ls -la /home/youruser/marketkit/api/secrets/firebase-service-account.json

# 2. Check file contents are valid JSON
jq . /home/youruser/marketkit/api/secrets/firebase-service-account.json

# 3. Ensure it contains "project_id"
jq '.project_id' /home/youruser/marketkit/api/secrets/firebase-service-account.json

# 4. Restart backend
docker compose down && docker compose up -d
```

### Issue: No Device Tokens in Database

**Cause:** Users haven't registered their devices (not logged in from mobile)

**Fix:**
1. Open mobile app
2. Login with test account
3. Accept notification permission when prompted
4. Verify token registered: 
   ```bash
   docker compose exec postgres psql -U admin -d marketkit -c "SELECT * FROM device_tokens WHERE user_id = 'test-user-id';"
   ```

### Issue: App Doesn't Receive Notifications

**Check:**
1. App must have notification permissions enabled (Settings → Notifications)
2. Device token must be registered in database
3. For Android: Must be Android 13+, permissions must include POST_NOTIFICATIONS
4. For iOS: Requires APNs key uploaded to Firebase Console (see FIREBASE_SETUP_GUIDE.md Step 1.6)

**Debug:**
```bash
# Mobile app logs - watch for:
# [FCM] Device token registered: abc123...
# [FCM] received message: title='Test'
```

### Issue: Notification Sent But Shows 0 Sent, 0 Failed

**Cause:** FCM service not properly initialized

**Fix:**
1. Verify credentials file exists
2. Restart backend
3. Check `notification_logs.sent_count` was updated:
   ```bash
   docker compose exec postgres psql -U admin -d marketkit -c "SELECT * FROM notification_logs ORDER BY created_at DESC LIMIT 1;"
   ```

---

## 📊 Database Queries for Diagnostics

### Check all registered devices
```sql
SELECT user_id, platform, COUNT(*) as device_count, MAX(updated_at) as last_updated
FROM device_tokens
GROUP BY user_id, platform
ORDER BY last_updated DESC;
```

### Check notification delivery stats
```sql
SELECT title, audience, sent_count, failed_count, created_at
FROM notification_logs
WHERE created_at > NOW() - INTERVAL '24 hours'
ORDER BY created_at DESC;
```

### Check per-user notifications
```sql
SELECT u.user_id, nl.title, nl.created_at, un.read
FROM user_notifications un
JOIN notification_logs nl ON un.notification_log_id = nl.id
WHERE u.created_at > NOW() - INTERVAL '1 day'
ORDER BY un.created_at DESC;
```

---

## 🚀 Next Steps After Verification

1. ✅ Place Firebase credentials file
2. ✅ Restart backend and verify FCM initialization
3. ✅ Test with at least 2 devices (Android + iOS if available)
4. ✅ Test different notification types:
   - Broadcast to all users
   - Targeted to specific plan subscribers
   - Targeted to active subscriptions only
5. ✅ Test notification actions (tap → deep link to gallery/community/home)
6. ✅ Monitor notification delivery rates and failed counts

---

## 📞 Support

If notifications still don't work after following this guide:

1. Check server logs: `docker compose logs api | grep -i fcm`
2. Check database: Verify device tokens and notification logs exist
3. Check mobile app logs in IDE console for `[FCM]` messages
4. Verify Firebase Console shows your project is correctly configured
