# Studio Manager Hub

A full-stack embroidery video learning platform for **Design Express**. Students subscribe to plans and watch video tutorials for Willcom, E4, and meCAD embroidery machines through the mobile app. Admins manage content, users, and payments through a web panel.

---

## Project Structure

```
marketkit/
├── api/          # Go backend — REST API, auth, payments, email
├── app/          # Flutter mobile app — "Stitch Craft Learn"
├── web /         # React admin panel — content & user management
├── docker-compose.yml
├── docker-compose.dev.yml
└── Makefile
```

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend API | Go 1.22, Fiber v2, GORM, PostgreSQL 16 |
| Mobile App | Flutter 3.11+, Riverpod, GoRouter, Chewie |
| Admin Panel | React 18, TypeScript, Vite, shadcn/ui, TanStack Query |
| Payments | Razorpay |
| Email | SMTP (gomail) with HTML templates |
| Infrastructure | Docker, Docker Compose |

---

## Architecture Overview

```
┌─────────────────┐     ┌─────────────────┐
│  Flutter App    │     │  React Admin    │
│ (Stitch Craft   │     │  Panel          │
│  Learn)         │     │  (web /)        │
└────────┬────────┘     └────────┬────────┘
         │                       │
         │    REST API (JSON)     │
         └──────────┬────────────┘
                    │
         ┌──────────▼────────────┐
         │   Go API (Fiber)      │
         │   /api/v1/...         │
         │                       │
         │  ┌─────────────────┐  │
         │  │  User Auth      │  │
         │  │  Admin Auth     │  │
         │  │  Videos         │  │
         │  │  Plans          │  │
         │  │  Payments       │  │
         │  │  Email / Cron   │  │
         │  └─────────────────┘  │
         └──────────┬────────────┘
                    │
         ┌──────────▼────────────┐
         │   PostgreSQL 16       │
         └───────────────────────┘
```

---

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Node.js 18+ (for admin panel)
- Flutter 3.11+ (for mobile app)
- Go 1.22+ (for local API development)

### 1. Clone and configure

```bash
git clone <repo-url>
cd marketkit
cp api/.env.example api/.env   # fill in your values
```

### 2. Start backend + database

```bash
make dev          # starts API + PostgreSQL + Mailhog (hot reload)
```

Or start everything at once (backend via Docker + admin panel via Vite):

```bash
make start
```

### 3. Seed the database

```bash
make seed         # creates default admin accounts and plans
```

### 4. Open the admin panel

```
http://localhost:5173
```

### 5. Run the Flutter app

```bash
cd app
flutter pub get
flutter run
```

---

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make dev` | Start backend in dev mode (Docker, hot reload) |
| `make start` | Start backend + admin panel together |
| `make stop` | Stop all backend containers |
| `make seed` | Seed database with admin and plans |
| `make web-dev` | Start admin panel dev server only |
| `make web-build` | Build admin panel for production |
| `make logs` | Tail logs for all services |
| `make db-shell` | Open psql shell in postgres container |
| `make down-v` | Stop containers and wipe database |
| `make fmt` | Format all Go source files |
| `make lint` | Run go vet |

## Live Commands

`make deploy`
`make apk`

---

## Environment Variables

Create `api/.env` with the following:

```env
# Database
DATABASE_URL=postgres://admin:password@localhost:5433/marketkit?sslmode=disable

# JWT
JWT_SECRET=your-secret-key-here
JWT_EXPIRY=8h

# SMTP (use Mailhog for local dev)
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

# Cloudflare R2 (optional — fill all R2_* vars for hybrid: new uploads → R2, legacy → disk)
# R2_BUCKET=
# R2_REGION=auto
# R2_ACCESS_KEY=
# R2_SECRET_KEY=
# R2_ENDPOINT=
# R2_PUBLIC_BASE=
```

See `api/.env.example` for the full list of variables (including `ADMIN_PASSWORD`,
`GOOGLE_CLIENT_SECRET`, `PUBLISH_SECRET`, `REDIS_URL`, etc.). Never commit a filled-in
`.env*` file — only `*.env.*.example` templates belong in git.

### ⚠️ Keys you must supply

This kit contains **no real credentials**. Every `.env*.example`,
`firebase-config.js.template`, and `secrets/*.example` file holds placeholders, and
`app/lib/firebase_options.dart` ships with dummy values so the project compiles before
you configure anything.

You need to create your own for:

| Service | Where it goes |
|---|---|
| Razorpay key ID / secret / webhook secret | `api/.env` |
| SMTP host + credentials | `api/.env` |
| `JWT_SECRET` (`openssl rand -hex 32`) | `api/.env` |
| `POSTGRES_PASSWORD` | root `.env` |
| Cloudflare R2 keys *(optional — falls back to local disk)* | `api/.env` |
| Google OAuth client ID / secret *(optional)* | `api/.env` + `--dart-define` |
| Firebase service account *(optional — push)* | `api/secrets/firebase-service-account.json` |
| Firebase app config *(optional — push)* | `flutterfire configure` in `app/` |

`.env` and `secrets/*.json` are gitignored by default. Keep it that way — a secret
committed even once stays in git history forever.

---

## Key Features

### Mobile App (Flutter)
- OTP-based authentication
- Browse and stream embroidery videos by category (Willcom / E4 / meCAD)
- Subscription plans with Razorpay payment
- Video player with Chewie controls
- Library of accessible videos
- Profile with subscription status

### Admin Panel (React)
- Dashboard with KPIs, charts, and recent activity
- Video upload and management (DRAFT → PROCESSING → PUBLISHED)
- User management with force logout and plan assignment
- Payment records (Razorpay + manual)
- Subscription plan configuration
- Session management and audit logs
- Revenue analytics and playback statistics

### Backend API
- Dual auth system — separate JWT flows for admins and users
- httpOnly refresh token cookies (XSS-safe)
- Video streaming with HTTP range request support
- Razorpay webhook with HMAC verification
- 8 transactional email templates
- Daily cron for subscription expiry checks
- Append-only audit log

---

## Sub-project READMEs

- [`api/README.md`](api/README.md) — Go backend setup and API reference
- [`app/README.md`](app/README.md) — Flutter mobile app setup
- [`web /README.md`](<web%20/README.md>) — React admin panel setup
