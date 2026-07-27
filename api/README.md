# MarketKit API

Go REST API for the MarketKit platform. Handles authentication, video management, subscription plans, Razorpay payments, and transactional email for both the Flutter mobile app and React admin panel.

---

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.22 |
| Framework | Fiber v2 |
| ORM | GORM |
| Database | PostgreSQL 16 |
| Auth | JWT (golang-jwt/jwt v5) |
| Email | gomail v2 + SMTP |
| Payments | Razorpay (via REST) |
| IDs | UUID v4 (google/uuid) |

---

## Project Structure

```
api/
├── cmd/api/
│   └── main.go                 # Entry point — wires all modules + cron
├── internal/
│   ├── config/                 # Env var loading + validation
│   ├── database/               # GORM connect + auto-migrate
│   ├── cron/                   # Daily subscription expiry checker
│   ├── email/
│   │   ├── mailer.go           # Send functions + shared template helper
│   │   └── templates/          # HTML email templates (8 total)
│   ├── middleware/             # JWT auth guards (admin + user)
│   ├── models/                 # GORM models + enums
│   └── modules/
│       ├── auth/               # Admin OTP login / refresh / logout
│       ├── audit_logs/         # Append-only admin action log
│       ├── dashboard/          # KPI aggregates for admin panel
│       ├── payments/           # Razorpay webhook + manual payments
│       ├── photos/             # Photo upload + management
│       ├── plans/              # Subscription plan CRUD
│       ├── playback/           # Video play-event logging
│       ├── revenue/            # Revenue analytics
│       ├── sessions/           # Admin session management
│       ├── user_auth/          # User OTP login / refresh / logout / me
│       ├── user_payments/      # User-side Razorpay order creation
│       ├── user_videos/        # User video list + playback log
│       ├── users/              # Admin user management
│       └── videos/             # Video upload + management (admin)
├── pkg/response/               # Shared JSON response helpers
├── uploads/                    # File storage (videos, photos)
├── go.mod
└── .env.example
```

---

## Setup

### Prerequisites

- Go 1.22+
- PostgreSQL 16 running on port 5433 (or use Docker Compose)

### Local (without Docker)

```bash
cd api
cp .env.example .env        # fill in values
go run ./cmd/api
```

### Docker (recommended)

```bash
# From repo root
make dev                    # starts API + PostgreSQL + Mailhog with hot reload
make seed                   # seed admin accounts and plans
```

---

## Environment Variables

```env
# Server
PORT=3000
CORS_ORIGIN=http://localhost:5173

# Database
DATABASE_URL=postgres://admin:password@localhost:5433/marketkit?sslmode=disable

# JWT
JWT_SECRET=your-secret-key-here
JWT_EXPIRY=8h

# SMTP (Mailhog for local dev)
SMTP_HOST=localhost
SMTP_PORT=1025
SMTP_SECURE=false
SMTP_USER=
SMTP_PASS=
SMTP_FROM=noreply@example.com

# Admin notifications
ADMIN_EMAIL=admin@example.com

# Razorpay
RAZORPAY_KEY_ID=rzp_test_xxxx
RAZORPAY_KEY_SECRET=xxxx
RAZORPAY_WEBHOOK_SECRET=xxxx

# Storage
UPLOAD_DIR=./uploads
MAX_FILE_SIZE_BYTES=4294967296
```

See `.env.example` in this directory for the complete list of variables (also
covers `ADMIN_PASSWORD`, Google OAuth, Cloudflare R2, `REDIS_URL`, `PUBLISH_SECRET`).
Only `.env.*.example` files should ever be committed — real `.env`/`.env.live`/
`.env.test` files stay local and are gitignored.

`ADMIN_PASSWORD` has no default: `SeedAdmin()` refuses to create the initial admin
account (and the process exits) if it isn't set, so it must be provided explicitly
on first boot.

### ⚠️ Before you go live

This kit ships with **no real credentials** — every `.env*.example` and
`secrets/*.example` file contains placeholders only. Before deploying:

1. Copy `.env.example` → `.env` and fill in every value.
2. Generate a strong `JWT_SECRET`: `openssl rand -hex 32`.
3. Add your own Razorpay keys, SMTP credentials, and (if used) Cloudflare R2 keys.
4. Drop your Firebase service account at `secrets/firebase-service-account.json`
   (see `secrets/firebase-service-account.json.example` for the shape).
5. Confirm `.env` and `secrets/*.json` are gitignored — they are by default. Never
   commit a filled-in one; a secret committed even once stays in git history forever.

---

## API Reference

All endpoints are under `/api/v1`. Protected routes require `Authorization: Bearer <token>`.

### Admin Auth — `/auth`

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/auth/send-otp` | — | Send OTP to admin email |
| POST | `/auth/verify-otp` | — | Verify OTP → access + refresh tokens |
| POST | `/auth/refresh` | cookie | Rotate refresh token |
| POST | `/auth/logout` | Admin | Revoke refresh token |
| GET | `/auth/me` | Admin | Current admin profile |

### User Auth — `/user/auth`

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/user/auth/register` | — | Register new user |
| POST | `/user/auth/send-otp` | — | Send OTP to user email |
| POST | `/user/auth/verify-otp` | — | Verify OTP → access + refresh tokens |
| POST | `/user/auth/refresh` | cookie | Rotate refresh token |
| POST | `/user/auth/logout` | User | Revoke refresh token |
| GET | `/user/auth/me` | User | Profile + active subscription |

`GET /user/auth/me` response includes:
```json
{
  "id": "...",
  "name": "...",
  "email": "...",
  "phone": "...",
  "status": "ACTIVE",
  "subscription": {
    "id": "...",
    "plan_id": "...",
    "plan_name": "All Access",
    "status": "ACTIVE",
    "expires_at": "2026-06-01T00:00:00Z",
    "features": ["CATEGORY_A", "CATEGORY_B"]
  }
}
```

### Users — `/users` (Admin)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/users` | Admin | List users (page, limit, search, status) |
| POST | `/users` | Admin | Create user with optional plan |
| GET | `/users/:id` | Admin | Get user + subscriptions |
| PATCH | `/users/:id` | Admin | Update name or status |
| DELETE | `/users/:id` | Admin | Delete user |
| POST | `/users/:id/force-logout` | Admin | Revoke all sessions |
| PATCH | `/users/:id/change-plan` | Admin | Assign new plan |

### Videos — `/videos` (Admin)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/videos` | Admin | List videos |
| POST | `/videos` | Admin | Upload video |
| GET | `/videos/:id` | Admin | Get video details |
| PATCH | `/videos/:id` | Admin | Update video metadata |
| DELETE | `/videos/:id` | Admin | Delete video |
| POST | `/videos/:id/retry` | Admin | Retry failed processing |

### User Videos — `/user/videos`

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/user/videos` | User | List videos with `accessible` flag |
| GET | `/user/videos/:id/stream` | User | Stream video (HTTP range, `?token=` fallback) |
| POST | `/user/videos/:id/playback-log` | User | Log playback event |

### Plans

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/plans/public` | — | List active plans (Flutter) |
| GET | `/plans` | Admin | List all plans |
| POST | `/plans` | Admin | Create plan |
| PATCH | `/plans/:id` | Admin | Update plan |
| DELETE | `/plans/:id` | Admin | Delete plan |

### Payments

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/payments/razorpay-webhook` | HMAC | Razorpay event receiver |
| GET | `/payments` | Admin | List payments |
| GET | `/payments/:id` | Admin | Get payment |
| POST | `/payments/manual` | Admin | Create manual payment + subscription |
| POST | `/payments/:id/activate` | Admin | Activate pending payment |
| POST | `/user/payments/order` | User | Create Razorpay order (Flutter checkout) |

### Other Admin Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/dashboard` | KPI aggregates |
| GET | `/revenue` | Revenue analytics |
| GET | `/audit-logs` | Admin action log |
| GET | `/sessions` | Admin session list |
| GET | `/playback` | Playback statistics |
| GET | `/photos` | Photo list |
| POST | `/photos` | Upload photo |

---

## Authentication Design

Two completely separate JWT flows share the same PostgreSQL database:

```
Admin flow:  POST /auth/verify-otp  →  JWT (8h)  +  httpOnly cookie (refresh)
User flow:   POST /user/auth/verify-otp  →  JWT (8h)  +  httpOnly cookie (refresh)
```

- Access tokens are short-lived Bearer tokens sent in `Authorization` header
- Refresh tokens are stored in httpOnly cookies (XSS-safe) and rotated on every use
- First-ever login (no prior refresh tokens) triggers a Welcome email

---

## Email Templates

Located in `api/internal/email/templates/`:

| Template | Trigger |
|----------|---------|
| `otp.html` | Every OTP send |
| `welcome.html` | First login (admin + user) |
| `payment_receipt.html` | Successful payment |
| `subscription_confirm.html` | New subscription or upgrade |
| `expiry_warning.html` | 6–8 days before expiry (cron) |
| `subscription_expired.html` | Subscription expired (cron) |
| `account_suspended.html` | Admin suspends user |
| `admin_sub_alert.html` | Admin notification on any subscription activation |

---

## Cron Jobs

`cron.StartSubscriptionChecker()` runs as a goroutine from `main.go`. It fires once on startup, then every 24 hours:

1. **Expiry warning** — emails users whose subscription expires in 6–8 days
2. **Mark expired** — updates `status=EXPIRED` and emails affected users

The 6–8 day window prevents duplicate emails if the cron fires slightly early or late.

---

## File Uploads

Videos and photos are stored in `UPLOAD_DIR` (default `./uploads`) and served as static files at `/uploads/*`. Maximum upload size is controlled by `MAX_FILE_SIZE_BYTES` (default 4 GB).

Video streaming (`GET /user/videos/:id/stream`) supports HTTP range requests for seek support. The `?token=` query parameter allows streaming directly from `<video>` HTML elements that cannot set Authorization headers.
