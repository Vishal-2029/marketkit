# Swagger + cURL API Docs (Request & Response Bodies)

Base URL: `http://localhost:3000/api/v1`

This document summarizes the Swagger-defined endpoints (from `api/docs/swagger.json`) and provides:
- Example **cURL** request
- Expected **request body** (for JSON and multipart)
- Example **response body**

> Note: Many Swagger responses are typed as `object` with `additionalProperties: true`. In those cases, example bodies are based on the response payloads implemented in handlers (or related model schemas), but the API may return additional fields.

## Auth Headers
- **Admin / User JWT** are sent via:
  - `Authorization: Bearer <token>`

## Admin Auth (cookie-based refresh)
- Refresh token is stored as an HTTPOnly cookie: `refresh_token`
- Access token is returned in JSON (see examples below).

---

## 1) Admin Auth

### POST `/auth/register`
Register a new **admin** account.

**cURL**
```bash
curl -sS -X POST 'http://localhost:3000/api/v1/auth/register' \
  -H 'Content-Type: application/json' \
  -d '{
    "first_name":"John",
    "last_name":"Doe",
    "phone":"9999999999",
    "email":"john@example.com",
    "password":"secret123"
  }'
```

**Request body (JSON)**
```json
{
  "first_name": "John",
  "last_name": "Doe",
  "phone": "9999999999",
  "email": "john@example.com",
  "password": "secret123"
}
```

**Response (200)**
```json
{
  "message": "Account created. Check your email for the OTP to verify."
}
```

---

### POST `/auth/send-otp`
Send OTP to admin email.

**cURL**
```bash
curl -sS -X POST 'http://localhost:3000/api/v1/auth/send-otp' \
  -H 'Content-Type: application/json' \
  -d '{
    "email":"john@example.com",
    "password":"secret123"
  }'
```

**Request body (JSON)**
```json
{
  "email": "john@example.com",
  "password": "secret123"
}
```

**Response (200)**
```json
{ "message": "OTP sent to your email." }
```

---

### POST `/auth/verify-otp`
Verify OTP and login as admin.

**cURL**
```bash
curl -sS -X POST 'http://localhost:3000/api/v1/auth/verify-otp' \
  -H 'Content-Type: application/json' \
  -d '{
    "email":"john@example.com",
    "otp":"123456"
  }'
```

**Request body (JSON)**
```json
{
  "email": "john@example.com",
  "otp": "123456"
}
```

**Response (200)**
```json
{
  "token": "<access_token>",
  "admin": {
    "id": "<admin_id>",
    "email": "john@example.com",
    "first_name": "John",
    "last_name": "Doe",
    "phone": "9999999999"
  }
}
```

> Refresh token is set via cookie `refresh_token`.

---

### POST `/auth/refresh`
Refresh admin access token (uses refresh cookie).

**cURL**
```bash
curl -sS -X POST 'http://localhost:3000/api/v1/auth/refresh' \
  --cookie 'refresh_token=<refresh_token>'
```

**Response (200)**
```json
{
  "token": "<new_access_token>",
  "admin": {
    "id": "<admin_id>",
    "email": "john@example.com",
    "first_name": "John",
    "last_name": "Doe",
    "phone": "9999999999"
  }
}
```

---

### POST `/auth/logout`
Logout admin (clears refresh cookie).

**cURL**
```bash
curl -sS -X POST 'http://localhost:3000/api/v1/auth/logout' \
  --cookie 'refresh_token=<refresh_token>'
```

**Response (200)**
```json
{ "message": "logged out" }
```

---

### GET `/auth/me`
Get current admin profile.

**cURL**
```bash
curl -sS -X GET 'http://localhost:3000/api/v1/auth/me' \
  -H 'Authorization: Bearer <admin_access_token>'
```

**Response (200)**
```json
{
  "id": "<admin_id>",
  "email": "john@example.com",
  "first_name": "John",
  "last_name": "Doe",
  "phone": "9999999999"
}
```

---

## 2) Admin Users

All these endpoints require **AdminAuth** header:
`Authorization: Bearer <admin_access_token>`

### POST `/admin/users`
Create a new user.

**cURL**
```bash
curl -sS -X POST 'http://localhost:3000/api/v1/admin/users' \
  -H 'Authorization: Bearer <admin_access_token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "name":"Alice",
    "email":"alice@example.com",
    "phone":"8888888888",
    "plan_id":"<optional_plan_id>"
  }'
```

**Request body (JSON)**
```json
{
  "name": "Alice",
  "email": "alice@example.com",
  "phone": "8888888888",
  "plan_id": "<optional_plan_id>"
}
```

**Response (201)**
Example (based on Swagger model `models.User`):
```json
{
  "id": "<user_id>",
  "email": "alice@example.com",
  "name": "Alice",
  "phone": "8888888888",
  "avatar_url": "<optional>",
  "status": "ACTIVE",
  "joined_at": "2026-01-01T00:00:00Z",
  "updated_at": "2026-01-01T00:00:00Z",
  "subscriptions": [],
  "payments": [],
  "playback_logs": [],
  "sessions": []
}
```

---

### PUT `/admin/users/{id}`
Update user details.

**cURL**
```bash
curl -sS -X PUT 'http://localhost:3000/api/v1/admin/users/<user_id>' \
  -H 'Authorization: Bearer <admin_access_token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Alice Updated",
    "status": "ACTIVE"
  }'
```

**Request body (JSON)**
```json
{
  "name": "Alice Updated",
  "status": "ACTIVE"
}
```

**Response (200)**
Returns the updated `models.User`.

---

### POST `/admin/users/{id}/plan`
Change a user’s subscription plan.

**cURL**
```bash
curl -sS -X POST 'http://localhost:3000/api/v1/admin/users/<user_id>/plan' \
  -H 'Authorization: Bearer <admin_access_token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "plan_id": "<new_plan_id>"
  }'
```

**Request body (JSON)**
```json
{ "plan_id": "<new_plan_id>" }
```

**Response (200)**
Swagger indicates a generic JSON object.

---

---

## 3) Admin Videos (multipart)

### POST `/admin/videos`
Create a new video.

**cURL**
```bash
curl -sS -X POST 'http://localhost:3000/api/v1/admin/videos' \
  -H 'Authorization: Bearer <admin_access_token>' \
  -F 'title=My Video' \
  -F 'description=Video description' \
  -F 'category=E4' \
  -F 'is_free=true' \
  -F 'is_preview=false' \
  -F 'file=@./video.mp4' \
  -F 'thumbnail=@./thumb.jpg' \
  -F 'thumbnail_url=https://example.com/thumb.jpg'
```

**Request body (multipart/form-data)**
Form fields:
- `title` (string, required)
- `description` (string)
- `category` (string, required)
- `is_free` (boolean)
- `is_preview` (boolean)
- `file` (file, required)
- `thumbnail` (file)
- `thumbnail_url` (string)

**Response (201)**
Returns `models.Video`.

---

### PUT `/admin/videos/{id}`
Update video details.

**cURL**
```bash
curl -sS -X PUT 'http://localhost:3000/api/v1/admin/videos/<video_id>' \
  -H 'Authorization: Bearer <admin_access_token>' \
  -F 'title=Updated Title' \
  -F 'description=Updated description' \
  -F 'category=E4' \
  -F 'is_free=false' \
  -F 'is_preview=true'
```

**Request body (multipart/form-data)**
Same fields as create (thumbnail optional).

**Response (200)**
Returns updated `models.Video`.

---

## 4) User Auth / Profile (multipart)

> These endpoints are under the user-auth router in code (`/user/auth/...`), but Swagger currently documents them under `/auth/...`. Use the Swagger paths below as the source of truth for this document.

### POST `/auth/avatar`
Upload profile picture.

**cURL**
```bash
curl -sS -X POST 'http://localhost:3000/api/v1/auth/avatar' \
  -H 'Authorization: Bearer <user_access_token>' \
  -F 'file=@./avatar.png'
```

**Request body (multipart/form-data)**
- `file` (file, required)

**Response (200)**
Swagger generic object; handler returns `response.OK` with JSON payload.

---

## 5) User Auth (JSON)

### PUT `/auth/profile`
Update user profile.

**cURL**
```bash
curl -sS -X PUT 'http://localhost:3000/api/v1/auth/profile' \
  -H 'Authorization: Bearer <user_access_token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Alice Updated",
    "phone": "8888880000"
  }'
```

**Request body (JSON)**
```json
{ "name": "Alice Updated", "phone": "8888880000" }
```

**Response (200)**
Swagger indicates a generic JSON object.

---

### PUT `/auth/change-password`
Change user password.

**cURL**
```bash
curl -sS -X PUT 'http://localhost:3000/api/v1/auth/change-password' \
  -H 'Authorization: Bearer <user_access_token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "current_password": "oldsecret",
    "new_password": "newsecret123"
  }'
```

**Request body (JSON)**
Swagger is generic; expected fields are typically:
- `current_password`
- `new_password`

**Response (200)**
Swagger indicates a generic JSON object.

---

### POST `/auth/device-token`
Register device token.

**cURL**
```bash
curl -sS -X POST 'http://localhost:3000/api/v1/auth/device-token' \
  -H 'Authorization: Bearer <user_access_token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "token": "<fcm_or_apns_token>",
    "platform": "android"
  }'
```

**Request body (JSON)**
Swagger is generic (`additionalProperties: true`).

**Response (200)**
Swagger indicates a generic JSON object.

---

## 6) Payments (admin, JSON)

### POST `/payments/manual`
Manually create a payment and activate subscription (admin).

**cURL**
```bash
curl -sS -X POST 'http://localhost:3000/api/v1/payments/manual' \
  -H 'Authorization: Bearer <admin_access_token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "user_id": "<user_id>",
    "plan_id": "<plan_id>",
    "notes": "Manual adjustment"
  }'
```

**Request body (JSON)**
Swagger is generic object.

**Response (201)**
Swagger indicates a generic JSON object.

---

## 7) Photos (admin, JSON)

### PUT `/photos/{id}`
Update a photo (admin).

**cURL**
```bash
curl -sS -X PUT 'http://localhost:3000/api/v1/photos/<photo_id>' \
  -H 'Authorization: Bearer <admin_access_token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Updated title",
    "category": "GENERAL",
    "is_published": true
  }'
```

**Request body (JSON)**
Swagger is generic object.

**Response (200)**
Swagger indicates a generic JSON object.

---

## File Added
- `docs/swagger-curl-api-docs.md`

