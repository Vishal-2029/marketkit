# Notification System Testing & Troubleshooting Guide

## System Overview

The notification system has 3 components:
1. **Admin Panel (Web)** - Sends notifications via `/admin/notifications/broadcast`
2. **Backend API** - Manages FCM delivery, stores device tokens, records notification logs
3. **Mobile App (Flutter)** - Receives notifications via Firebase Cloud Messaging (FCM)

---

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
