# MarketKit

A production-ready starter kit for marketplace apps: a Go backend, a Flutter mobile app, and a React admin panel that ship together. It includes the parts that normally take months — a wallet ledger, seller payouts, platform fees, refunds, and subscriptions.

---

## Project Structure

```
marketkit/
├── api/          # Go backend — REST API, auth, payments, email
├── app/          # Flutter mobile app
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
| Payments | Razorpay + Stripe (pluggable provider interface) |
| Email | SMTP (gomail) with HTML templates |
| Infrastructure | Docker, Docker Compose |

---

## Architecture Overview

```
┌─────────────────┐     ┌─────────────────┐
│  Flutter App    │     │  React Admin    │
│  (Flutter)      │     │  Panel          │
│                 │     │  (web/)         │
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

### One command

```bash
make quickstart
```

That generates `.env` and `api/.env` with fresh random secrets, builds and starts
the API + PostgreSQL + Redis + Mailhog, waits for the API to become healthy, and
fills the database with demo data — sellers, products, purchases, wallets and
platform revenue.

When it finishes:

| | |
|---|---|
| API | http://localhost:3000 |
| Swagger | http://localhost:3000/swagger/index.html |
| Mailhog (catches all email) | http://localhost:8025 |
| Admin panel | `make web-dev` → http://localhost:5173 |

Demo logins — password `demo1234` for all of them:

```
seller1@demo.marketkit.test   … seller5@demo.marketkit.test
buyer1@demo.marketkit.test    … buyer8@demo.marketkit.test
```

The admin password is random per install and printed during bootstrap; it is
also in `api/.env` as `ADMIN_PASSWORD`.

Stop everything with `make down`.

> Port 3000 already in use? Change `PORT` in the root `.env` and re-run.

### Run the Flutter app

```bash
cd app
flutter pub get
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:3000/api/v1   # Android emulator
```

### Useful commands

| Command | What |
|---|---|
| `make quickstart` | First run — bootstrap, start, seed demo data |
| `make bootstrap` | Regenerate missing `.env` files (never overwrites) |
| `make up` / `make down` | Start / stop the stack |
| `make seed-demo` | Add demo data to an existing database |
| `make seed-demo-reset` | Wipe demo data and re-seed |
| `make test` | Go test suite against a throwaway Postgres |
| `make swagger` | Regenerate the API docs |

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
- Browse and stream videos by category
- Subscription plans with in-app checkout (Razorpay or Stripe)
- Video player with Chewie controls
- Library of accessible videos
- Profile with subscription status

### Admin Panel (React)
- Dashboard with KPIs, charts, and recent activity
- Video upload and management (DRAFT → PROCESSING → PUBLISHED)
- User management with force logout and plan assignment
- Payment records (any gateway + manual)
- Subscription plan configuration
- Session management and audit logs
- Revenue analytics and playback statistics

### Backend API
- Wallet ledger with seller payouts and platform fee split
- Pluggable payment providers — add a gateway without touching the modules that take money
- Dual auth system — separate JWT flows for admins and users
- httpOnly refresh token cookies (XSS-safe)
- Video streaming with HTTP range request support
- Gateway webhooks with signature verification (fails closed)
- 8 transactional email templates
- Daily cron for subscription expiry checks
- Append-only audit log

---

## Sub-project READMEs

- [`api/README.md`](api/README.md) — Go backend setup and API reference
- [`app/README.md`](app/README.md) — Flutter mobile app setup
- [`web /README.md`](<web%20/README.md>) — React admin panel setup
