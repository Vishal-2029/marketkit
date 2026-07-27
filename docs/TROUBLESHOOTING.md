# MarketKit - Troubleshooting Guide

## If App Crashes on Startup

### Symptom: Gold splash screen appears, then app closes

**Cause:** Firebase initialization failure

**Fix:**
1. Uninstall app completely
2. Clear app cache: Settings → Apps → MarketKit → Storage → Clear Cache
3. Reinstall APK
4. If still crashes, check:
   ```bash
   adb logcat --pid=$(adb shell pidof com.example.marketkit) 2>/dev/null | grep -E "FATAL|Exception|Error"
   ```

---

## If Videos Don't Play

### Symptom: Video screen shows loading spinner forever

**Cause:** Network issue or video file missing

**Fix:**
1. Check internet connection (4G/WiFi)
2. Try a different video
3. Restart app
4. If still fails, check server:
   ```bash
   curl -I https://example.com/api/v1/user/videos/intro
   ```

---

## If Login Fails

### Symptom: "Invalid email or password" error

**Cause:** Wrong credentials or server issue

**Fix:**
1. Verify email is correct
2. Check password (case-sensitive)
3. Restart app
4. If server is down:
   ```bash
   curl https://example.com/api/v1/plans/public
   ```

---

## If Notifications Don't Arrive

### Symptom: No push notifications after login

**Cause:** Device token not registered

**Fix:**
1. Logout and login again
2. Check if token registered:
   ```bash
   docker exec marketkit-postgres-1 psql -U admin -d marketkit -c "SELECT * FROM device_tokens WHERE user_id='YOUR_USER_ID';"
   ```
3. If empty, check app logs for registration errors

---

## If Admin Panel is Slow

### Symptom: Dashboard takes >5 seconds to load

**Cause:** Database query slow or network latency

**Fix:**
1. Clear browser cache
2. Try incognito mode
3. Check server load:
   ```bash
   docker stats marketkit-api-1
   ```
4. Restart API if CPU >80%:
   ```bash
   docker restart marketkit-api-1
   ```

---

## If Video Upload Fails

### Symptom: Upload button doesn't work or shows error

**Cause:** File too large or storage full

**Fix:**
1. Check file size (max 4GB)
2. Check disk space:
   ```bash
   df -h /root/marketkit/api/uploads
   ```
3. If full, delete old videos or enable R2 storage

---

## Server Health Check Commands

```bash
# Check all containers
docker ps -a

# Check API logs
docker logs marketkit-api-1 --tail 50

# Check database
docker exec marketkit-postgres-1 psql -U admin -d marketkit -c "SELECT 1;"

# Check Redis
docker exec marketkit-redis-1 redis-cli ping

# Check disk space
df -h

# Check API response
curl -s https://example.com/api/v1/plans/public | head -20
```

---

## Quick Restart Procedures

### Restart API Only
```bash
docker restart marketkit-api-1
```

### Restart All Services
```bash
cd /root/marketkit
docker compose restart
```

### Full Reset (WARNING: Clears data)
```bash
cd /root/marketkit
docker compose down -v
docker compose up -d
```

---

## Contact Support

If issues persist:
1. Collect logs: `docker logs marketkit-api-1 > api.log`
2. Check device logs: `adb logcat > device.log`
3. Share both logs with support team
