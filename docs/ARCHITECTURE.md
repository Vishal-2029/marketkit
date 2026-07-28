# MarketKit — Full Architecture & Services Guide

This document covers every service, module, screen, and external integration in the platform.
It explains **what** each part does and **why** it exists.

---

## Table of Contents

1. [System Overview](#1-system-overview)
2. [Infrastructure Services](#2-infrastructure-services)
3. [Backend API](#3-backend-api)
   - [Authentication](#31-authentication)
   - [Videos](#32-videos)
   - [Photos](#33-photos)
   - [Plans & Subscriptions](#34-plans--subscriptions)
   - [Payments](#35-payments)
   - [Users](#36-users)
   - [Community](#37-community)
   - [Notifications](#38-notifications)
   - [Analytics & Dashboard](#39-analytics--dashboard)
   - [Sessions & Security](#310-sessions--security)
4. [Admin Web Panel](#4-admin-web-panel)
5. [Flutter Mobile App](#5-flutter-mobile-app)
6. [External Services](#6-external-services)
7. [Database Tables](#7-database-tables)
8. [File Storage](#8-file-storage)
9. [Background Jobs](#9-background-jobs)
10. [Environment Variables](#10-environment-variables)
11. [API Endpoints Reference](#11-api-endpoints-reference)

---

## 1. System Overview

**MarketKit** is a marketplace starter kit with three layers:

```
┌─────────────────────────────────────────────────────┐
│                Flutter Mobile App                    │
│         (Students watch videos, subscribe)           │
└────────────────────┬────────────────────────────────┘
                     │ REST API
┌────────────────────▼────────────────────────────────┐
│                React Admin Panel                     │
│      (Admins manage content, users, payments)        │
└────────────────────┬────────────────────────────────┘
                     │ REST API
┌────────────────────▼────────────────────────────────┐
│              Go + Fiber REST API (Port 3000)         │
│  ┌──────────┐  ┌──────────┐  ┌────────────────────┐ │
│  │PostgreSQL│  │  Redis   │  │  File Storage      │ │
│  │(Database)│  │ (Cache)  │  │ (Local or R2/S3)   │ │
│  └──────────┘  └──────────┘  └────────────────────┘ │
└─────────────────────────────────────────────────────┘
                     ▲
                     │ Reverse Proxy
┌────────────────────┴────────────────────────────────┐
│                  Nginx (Port 80)                     │
└─────────────────────────────────────────────────────┘
```

**Tech stack summary:**

| Layer | Technology | Why |
|---|---|---|
| Backend | Go + Fiber | Fast, handles thousands of concurrent connections, small memory footprint |
| Database | PostgreSQL 16 | Reliable relational DB, supports JSONB, UUIDs, complex queries |
| Cache | Redis 7 | In-memory cache to reduce DB load under high traffic |
| Reverse proxy | Nginx | Buffers slow clients, handles gzip, terminates connections before Go |
| Admin web | React + TypeScript + Vite | Fast SPA for admin dashboard with type safety |
| Mobile app | Flutter | Single codebase for Android and iOS |
| File storage | Cloudflare R2 (or local disk) | Zero egress fees, global CDN for video delivery |

---

## 2. Infrastructure Services

### 2.1 PostgreSQL (`postgres`)
**What it does:** Primary database. Stores all users, videos, subscriptions, payments, audit logs, and every other piece of data.

**Why PostgreSQL:**
- Handles complex JOIN queries (e.g., user + subscription + plan in one query)
- JSONB columns for flexible metadata (payment details, audit event details)
- UUID primary keys for all tables — safe to expose in URLs and APIs
- Full-text search with `ILIKE` for video title search
- ACID transactions for payment activation (critical for money operations)

**Configuration:**
```
Image: postgres:16-alpine
Data volume: pgdata (persists data between restarts)
Max connections used by API: 200 (set in database.go)
```

---

### 2.2 Redis (`redis`)
**What it does:** In-memory cache layer. Stores frequently-read data so the API does not hit PostgreSQL on every request.

**Why Redis:**
- Under 1000 concurrent users, the video list endpoint would hit PostgreSQL 1000+ times per second without caching
- Redis serves the same data from memory in microseconds instead of milliseconds
- `allkeys-lru` eviction policy: automatically removes least-recently-used keys when memory is full
- 256 MB memory limit — enough to cache thousands of video list responses

**What is cached:**
| Cache Key Pattern | TTL | What it stores |
|---|---|---|
| `videos:admin:{params}` | 60 seconds | Admin video list with filters/pagination |
| `videos:published:{page}:{category}` | 60 seconds | Published video list for all mobile users |

**Cache is invalidated when:** a video is created, updated, or deleted.

**App works without Redis:** if Redis is unavailable, every request falls back to PostgreSQL. No data is lost.

**Configuration:**
```
Image: redis:7-alpine
Memory limit: 256 MB
Eviction: allkeys-lru (removes old keys automatically)
Data volume: redisdata (cache survives restarts)
```

---

### 2.3 Nginx (`nginx`)
**What it does:** Sits in front of the Go API as a reverse proxy on port 80. All external traffic goes through Nginx first.

**Why Nginx:**
- **Connection buffering:** Slow mobile clients hold open connections. Nginx buffers these so Go's goroutines are released immediately after writing the response
- **Gzip compression:** Compresses JSON responses (API), reducing bandwidth usage by 60-80%
- **Large file uploads:** Handles 4GB video upload buffering (`client_max_body_size 4G`)
- **Keepalive:** Reuses connections to the Go API (64 keepalive connections), reducing TCP handshake overhead
- **Future SSL:** Adding HTTPS only requires adding SSL config to nginx.conf — no changes to Go code

**Configuration file:** `nginx/nginx.conf`

```
Port: 80 (external)
Worker connections: 4096 per worker (auto workers = CPU cores)
Upload limit: 4 GB
Proxy read timeout: 300 seconds (for large video uploads)
```

---

### 2.4 Go API (`api`)
**What it does:** The core backend. Handles all business logic, authentication, data storage, and communication with external services.

**Why Go + Fiber:**
- Go handles thousands of concurrent connections with very low memory (each goroutine ~8KB vs ~1MB per thread in other languages)
- Fiber is built on `fasthttp` — the fastest HTTP framework in Go
- With 1000 users, the API uses ~200 MB RAM total

**Configuration:**
```
Port: 3000 (internal), exposed via Nginx on port 80
CPU limit: 2 cores
Memory limit: 2 GB
Fiber ReadTimeout: 30s
Fiber WriteTimeout: 60s
Fiber IdleTimeout: 120s
DB pool: 200 open connections, 25 idle
```

---

## 3. Backend API

Base URL: `/api/v1`

All routes are split into two groups:
- **Admin routes** — require `Authorization: Bearer <admin_jwt>` header
- **User routes** — require `Authorization: Bearer <user_jwt>` header
- **Public routes** — no authentication (e.g. plans list, photos gallery)

---

### 3.1 Authentication

#### Admin Auth (`/api/v1/auth/`)
**What it does:** Manages admin login using a secure OTP (One-Time Password) sent to email. No password is stored for admins.

| Method | Route | Purpose |
|---|---|---|
| POST | `/auth/send-otp` | Sends a 6-digit OTP to the admin email |
| POST | `/auth/verify-otp` | Validates OTP and returns JWT + refresh token |
| POST | `/auth/refresh` | Gets a new JWT using a refresh token |
| POST | `/auth/logout` | Revokes the refresh token |
| GET | `/auth/me` | Returns the currently logged-in admin's profile |

**Why OTP instead of password:**
- Admin panel is used rarely — remembering a password is unnecessary
- OTP expires in 10 minutes — harder to brute-force
- If admin email is secure, the system is secure

**Tokens issued:**
- **Access token (JWT):** Valid for `JWT_EXPIRY` (default 8 hours). Used for every API request.
- **Refresh token:** Long-lived, stored in DB. Used to get new access tokens without re-logging in.

---

#### User Auth (`/api/v1/user/auth/`)
**What it does:** Manages student registration, login, profile, and password management.

| Method | Route | Purpose |
|---|---|---|
| POST | `/user/auth/register` | Create new student account |
| POST | `/user/auth/send-otp` | Send OTP for login or password reset |
| POST | `/user/auth/verify-otp` | Login with OTP (returns JWT) |
| POST | `/user/auth/forgot-password` | Request password reset |
| POST | `/user/auth/reset-password` | Set new password using reset token |
| POST | `/user/auth/refresh` | Refresh access token |
| POST | `/user/auth/logout` | Logout (revokes token) |
| GET | `/user/auth/me` | Get own profile |
| PATCH | `/user/auth/me` | Update name, phone |
| POST | `/user/auth/avatar` | Upload profile photo |
| POST | `/user/auth/change-password` | Change password (requires current password) |
| POST | `/user/auth/device-token` | Register FCM device token for push notifications |

**Single-device session enforcement:** Each login creates a `current_session_id`. If the same account logs in on another device, the previous session is invalidated. The middleware detects this and returns `401 Unauthorized` on the old device.

---

### 3.2 Videos

#### Admin Videos (`/api/v1/videos/`)
**What it does:** Full CRUD for video content management.

| Method | Route | Purpose |
|---|---|---|
| GET | `/videos/` | List all videos (paginated, filterable by search/category/status/is_free) |
| POST | `/videos/create` | Upload a new video (multipart: file + thumbnail + metadata) |
| GET | `/videos/:id` | Get a single video's details |
| PATCH | `/videos/:id` | Edit video (title, description, status, thumbnail, is_free, is_preview) |
| DELETE | `/videos/:id` | Delete video and remove file from storage |
| POST | `/videos/:id/retry` | Re-queue a failed video for processing |
| GET | `/videos/:id/stream` | Generate time-limited streaming URL (redirects to storage) |
| GET | `/videos/:id/stream-url` | Same as above but returns URL as JSON |

**Video statuses:**
- `DRAFT` — uploaded but not visible to students
- `PROCESSING` — being encoded or validated
- `PUBLISHED` — visible to students with appropriate subscription
- `ERROR` — upload/processing failed, can retry

**Video categories:** `CATEGORY_A`, `CATEGORY_B`, `CATEGORY_C` — each doubles as a plan feature key. Rename them in `api/internal/models/video.go`.

**Streaming:** Videos are never served directly by the API. The API generates a **signed URL** with a 2-hour expiry. The client (mobile app or browser) uses that URL to stream directly from storage (R2/S3). This means the API handles no video bandwidth.

---

#### User Videos (`/api/v1/user/videos/`)
**What it does:** Student-facing video browsing, access control, and progress tracking.

| Method | Route | Purpose |
|---|---|---|
| GET | `/user/videos/` | List published videos with subscription access flags |
| GET | `/user/videos/:id/stream` | Get streaming URL (validates subscription access first) |
| GET | `/user/videos/:id/stream-url` | Same as above, returns JSON URL |
| POST | `/user/videos/:id/playback-log` | Record a video view (for analytics) |
| POST | `/user/videos/:id/progress` | Save current watch position |

**Access control logic:**
- `is_free = true` → all logged-in users can watch
- `is_preview = true` → all logged-in users can watch (marketing)
- Otherwise → user needs an active subscription whose plan covers that video's category
  - A video is accessible when its category key appears in `Plan.features`

---

### 3.3 Photos

#### Public Gallery (`/api/v1/photos/public`)
**What it does:** Returns all published photos — no authentication required. Used by the mobile app's gallery screen.

#### Admin Photos (`/api/v1/photos/`)
| Method | Route | Purpose |
|---|---|---|
| GET | `/photos/` | List all photos (admin, includes unpublished) |
| POST | `/photos/create` | Upload new photo (max 10 MB, auto-converted to JPEG) |
| PATCH | `/photos/:id` | Update title, category, or publish status |
| DELETE | `/photos/:id` | Delete photo and remove file from storage |

---

### 3.4 Plans & Subscriptions

#### Public Plans (`/api/v1/plans/public`)
Returns all active subscription plans — no authentication. Used by the mobile app's plans/pricing screen.

#### Admin Plans (`/api/v1/plans/`)
| Method | Route | Purpose |
|---|---|---|
| GET | `/plans/` | List all plans |
| POST | `/plans/create` | Create a new subscription plan |
| PATCH | `/plans/:id` | Update plan pricing, duration, features |
| DELETE | `/plans/:id` | Delete a plan |

**Plan feature flags:** Each plan can have one or more categories enabled:
- `features` — string array of feature keys the plan grants



**Pricing:** Stored in minor units (1 rupee = 100 minor units) to avoid floating-point issues.
`duration_days` controls how long the subscription lasts after activation.

---

### 3.5 Payments

#### User Payments (`/api/v1/user/payments/`)
| Method | Route | Purpose |
|---|---|---|
| POST | `/user/payments/order` | Create a Razorpay payment order. Returns `order_id` to the mobile app |

**How the payment flow works:**
1. Mobile app calls `POST /user/payments/order` with `plan_id`
2. API creates a Razorpay order and returns `order_id` + `amount`
3. Mobile app opens Razorpay checkout with this `order_id`
4. Student completes payment in Razorpay UI
5. Razorpay sends a `payment.captured` webhook to the API
6. API verifies the HMAC signature and activates the subscription
7. API sends payment receipt email + push notification to the student

#### Admin Payments (`/api/v1/payments/`)
| Method | Route | Purpose |
|---|---|---|
| POST | `/payments/webhook` | Receives gateway payment events (signature-verified). `/payments/razorpay-webhook` remains as a legacy alias |
| GET | `/payments/` | List all payment records |
| GET | `/payments/:id` | Get single payment details |
| POST | `/payments/manual` | Create a manual payment record (cash, bank transfer) |
| POST | `/payments/:id/activate` | Manually activate a subscription for a payment |

**HMAC Verification:** Every webhook from Razorpay includes a signature. The API verifies it using `RAZORPAY_WEBHOOK_SECRET` before processing. If the signature doesn't match, the request is rejected with 400.

---

### 3.6 Users

#### Admin User Management (`/api/v1/users/`)
| Method | Route | Purpose |
|---|---|---|
| GET | `/users/` | List all users (paginated, searchable) |
| POST | `/users/create` | Create a user account manually |
| GET | `/users/:id` | Get user profile + subscription status |
| PATCH | `/users/:id` | Update user details |
| DELETE | `/users/:id` | Delete user account and all associated data |
| POST | `/users/:id/force-logout` | Revoke all active sessions (used for security incidents) |
| PATCH | `/users/:id/suspend` | Suspend or reinstate a user account |
| POST | `/users/:id/change-plan` | Assign a subscription plan to a user manually |

---

### 3.7 Community

**What it does:** A forum where students can post questions, share work, and reply to each other. Posts are anonymous by default (shown with `anon_name` generated by the system).

#### User Community (`/api/v1/user/community/`)
| Method | Route | Purpose |
|---|---|---|
| GET | `/user/community/posts` | Browse community posts (paginated) |
| POST | `/user/community/posts` | Create a new post (supports image attachments, 5 MB each) |
| GET | `/user/community/posts/:id` | View a single post |
| GET | `/user/community/posts/:id/replies` | List replies to a post |
| POST | `/user/community/posts/:id/replies` | Add a reply (supports image) |

#### Admin Community Moderation (`/api/v1/community/`)
| Method | Route | Purpose |
|---|---|---|
| GET | `/community/posts` | View all posts including flagged ones |
| POST | `/community/posts` | Create post as admin |
| POST | `/community/posts/:id/replies` | Add admin reply |
| PATCH | `/community/posts/:id` | Edit post content |
| DELETE | `/community/posts/:id` | Delete a post and all its replies |
| DELETE | `/community/replies/:id` | Delete a single reply |
| PATCH | `/community/posts/:id/flag` | Flag or unflag a post for review |

**Rate limiting:**
- New posts: 3 per 10 minutes per user (prevents spam)
- Replies: 10 per 10 minutes per user

---

### 3.8 Notifications

#### Admin Push Notifications (`/api/v1/admin/notifications/`)
| Method | Route | Purpose |
|---|---|---|
| GET | `/admin/notifications/` | List all broadcast history |
| POST | `/admin/notifications/broadcast` | Send push notification to users |

**Broadcast targets:**
- **All users** — when no `plan_id` or `sub_status` is specified
- **Segment** — filter by `plan_id` and/or `sub_status` (e.g. only active subscribers on Plan A)

**Async processing:** When an admin triggers a broadcast, the API returns immediately with `"notification queued"`. The FCM send loop runs in a background goroutine so the admin's browser doesn't wait.

#### User Notifications (`/api/v1/user/notifications/`)
| Method | Route | Purpose |
|---|---|---|
| GET | `/user/notifications/` | Get inbox (unread notifications) |
| POST | `/user/notifications/read` | Mark notifications as read |

---

### 3.9 Analytics & Dashboard

#### Dashboard (`/api/v1/dashboard/`)
Used by the admin panel's home page.

| Route | What it returns |
|---|---|
| `GET /dashboard/stats` | Total users, active subscribers, total revenue, total videos |
| `GET /dashboard/recent-payments` | Last 10 payments |
| `GET /dashboard/recent-activity` | Last 20 audit log entries |
| `GET /dashboard/user-growth` | Monthly new user signups (chart data) |
| `GET /dashboard/plan-distribution` | How many users are on each plan |

#### Playback Analytics (`/api/v1/playback/`)

| Route | What it returns |
|---|---|
| `GET /playback/top-videos` | Most-watched videos by view count |
| `GET /playback/by-user` | Watch time per user |
| `GET /playback/summary` | Total views, watch hours, completion rate |
| `GET /playback/daily-trend` | Daily view counts (chart data) |

#### Revenue Analytics (`/api/v1/revenue/`)

| Route | What it returns |
|---|---|
| `GET /revenue/summary` | Total revenue, average order value, refund rate |
| `GET /revenue/monthly` | Revenue by month (chart data) |
| `GET /revenue/by-plan` | Revenue breakdown by subscription plan |
| `GET /revenue/forecast` | Predicted next-month revenue based on renewals |
| `GET /revenue/renewal-stats` | Upcoming subscription renewals |

---

### 3.10 Sessions & Security

#### Session Management (`/api/v1/sessions/`)
| Method | Route | Purpose |
|---|---|---|
| GET | `/sessions/` | List all active sessions for the current user |
| DELETE | `/sessions/` | Revoke all sessions (logout everywhere) |
| DELETE | `/sessions/:id` | Revoke one specific session |

#### Video Engagement (`/api/v1/user/videos/:id/reactions`, `/api/v1/user/videos/:id/comments`)
| Method | Route | Purpose |
|---|---|---|
| GET | `/:id/reactions` | Get like/dislike counts for a video |
| POST | `/:id/reactions` | Like or dislike a video |
| GET | `/:id/comments` | List comments on a video |
| POST | `/:id/comments` | Add a comment |
| DELETE | `/comments/:cid` | Delete own comment |

#### Audit Logs (`/api/v1/audit-logs/`)
Append-only event log. Every important action (login, subscription, payment, suspension, deletion) is recorded here with IP address, actor ID, and details. Cannot be edited or deleted.

Events logged: `admin_login`, `admin_logout`, `user_login`, `user_logout`, `user_registered`, `plan_changed`, `subscription_created`, `payment_activated`, `user_suspended`, `user_deleted`, `video_deleted`, `video_uploaded`.

---

## 4. Admin Web Panel

**Technology:** React 18 + TypeScript + Vite + shadcn/ui + TanStack Query + Recharts

**Access:** Runs in a browser. Requires admin login (OTP via email).

**URL:** `http://your-server-ip:8080` (dev) or `http://your-server/` (production behind Nginx)

### Pages

| Page | URL | Purpose |
|---|---|---|
| **Login** | `/login` | OTP-based admin login |
| **Dashboard** | `/` | KPI cards, user growth chart, plan distribution pie chart, recent payments table, recent activity feed |
| **Videos** | `/videos` | Upload videos, set status, edit title/thumbnail, delete. Shows file size, upload date, category |
| **Photos** | `/photos` | Upload gallery images, publish/unpublish, delete |
| **Users** | `/users` | Browse all students. Create, edit, suspend, force-logout, assign plan |
| **Plans** | `/plans` | Create and edit subscription plans. Set pricing, duration, category access |
| **Payments** | `/payments` | View all transactions. Activate manual payments. See Razorpay vs manual gateway split |
| **Revenue** | `/revenue` | Revenue charts: monthly trend, by-plan breakdown, renewal forecast |
| **Playback** | `/playback` | Which videos are watched most, user watch time, daily view trend |
| **Community** | `/community` | View all forum posts. Flag inappropriate content. Delete posts/replies |
| **Notifications** | `/notifications` | Broadcast push notifications. View broadcast history |
| **Sessions** | `/sessions` | View active admin sessions. Revoke if needed |
| **Audit Logs** | `/audit-logs` | Full event history with filters (event type, date range, actor) |

### Why this stack:
- **React + TypeScript** — Type-safe components, catches bugs at compile time
- **Vite** — Extremely fast dev server and build (10x faster than CRA)
- **shadcn/ui** — Pre-built accessible components (tables, dialogs, forms) that match a professional admin look
- **TanStack Query** — Handles API caching, loading states, and error states out of the box
- **Recharts** — Lightweight chart library for revenue and analytics graphs

---

## 5. Flutter Mobile App

**Technology:** Flutter 3 + Dart + Riverpod + GoRouter + Chewie (video player)

**Target platforms:** Android and iOS from a single codebase

**App name:** MarketKit

### Screens & Features

#### Authentication
| Screen | Purpose |
|---|---|
| Splash | Shows logo, checks if user is already logged in |
| Login | Email + OTP or email + password login |
| Register | Create new student account |
| OTP | Enter the 6-digit OTP received by email |
| Forgot Password | Request password reset email |
| Reset Password | Set new password |

#### Home
The home screen shows:
- Featured videos (previews/free videos)
- Subscription status banner
- Quick category navigation
- Recent community activity

#### Library
- Browse all published videos
- Filter by category
- Locked icon on videos the user doesn't have access to
- Tap a locked video → redirected to Plans screen

#### Video Player
- Full-screen video player using **Chewie** (built on `video_player`)
- Progress bar, play/pause, seek, fullscreen toggle
- Automatically saves watch position every 30 seconds
- Logs a playback event when video starts (for analytics)
- Like/dislike buttons
- Comments section below video

#### Plans & Subscription
- Shows all active subscription plans fetched from `GET /api/v1/plans/public`
- Displays price, duration, and which categories are included
- Tapping "Subscribe" opens Razorpay checkout
- After payment: subscription is activated via webhook and app refreshes

#### Photos Gallery
- Grid view of published photos from `GET /api/v1/photos/public`
- Full-screen viewer on tap
- Filter by category

#### Community
- Browse forum posts
- Create new post (text + optional image)
- View post details and replies
- Add reply

#### Profile
- View subscription status and expiry date
- Edit name, phone
- Change avatar (uploads to API)
- Change password
- View notification inbox
- Logout

### Why Flutter:
- One codebase → Android APK and iOS IPA from the same code
- Dart compiles to native ARM code — same performance as native Java/Swift
- Chewie video player supports HLS, byte-range streaming, and R2/S3 signed URLs
- Riverpod state management: reactive, testable, no boilerplate
- GoRouter: deep linking support (e.g. open a specific video from a push notification)

---

## 6. External Services

### 6.1 Payment gateways

Gateways sit behind a `Provider` interface (`internal/payments/provider`), so
the modules that take money — plans, marketplace purchases, wallet top-ups,
refunds — never reference a specific gateway.

**Shipped implementations:** Razorpay and Stripe. Select one with
`PAYMENT_PROVIDER` in `api/.env`.

**Adding a gateway** means implementing five methods and registering it in
`internal/payments/payments.go`:

```go
Name() / CreateOrder() / VerifyCheckout() / ParseWebhook() / Refund()
```

**Payment flow:**

1. API creates an order/PaymentIntent through the active provider
2. The endpoint returns a provider-neutral payload: `provider`, `order_id`,
   `amount_minor`, `currency`, `public_key`, and `client_secret` when the
   gateway needs one
3. The app opens the matching checkout sheet
   (`app/lib/core/payments/checkout.dart`)
4. The gateway sends a webhook to `POST /api/v1/payments/webhook`
5. The provider verifies the signature and normalizes the event
6. `internal/payments/capture` dispatches it to whichever module owns the order
   — learning plan, marketplace purchase, market plan, or wallet top-up

**Security properties both implementations honour:**

- Webhook verification **fails closed**: an unconfigured signing secret rejects
  every request rather than accepting unsigned ones
- Stripe additionally enforces a 5-minute timestamp tolerance, so a captured
  request cannot be replayed
- Stripe returns no client-side signature, so `VerifyCheckout` asks Stripe
  whether the PaymentIntent actually succeeded rather than trusting the client

**Environment:**

```env
PAYMENT_PROVIDER=razorpay        # or stripe
PAYMENT_CURRENCY=INR

RAZORPAY_KEY_ID=
RAZORPAY_KEY_SECRET=
RAZORPAY_WEBHOOK_SECRET=

STRIPE_SECRET_KEY=
STRIPE_PUBLISHABLE_KEY=
STRIPE_WEBHOOK_SECRET=
```

See [WALLET.md](WALLET.md) for what happens to the money after capture.

### 6.2 Firebase Cloud Messaging (FCM)

**What it does:** Sends push notifications to Android and iOS devices.

**Why FCM:**
- Free, unlimited push notifications
- Works for both Android (native) and iOS (via APNs bridge)
- No server needed — Google handles delivery
- Supports background notifications (received even when app is closed)

**How it's used:**
- Student installs app → registers device token via `POST /user/auth/device-token`
- Tokens stored in `device_tokens` table
- Admin broadcasts notification → API sends FCM request for each token
- FCM delivers notification to device
- Notifications also stored in `user_notifications` table for in-app inbox

**Automatic triggers (no admin action needed):**
- When a new video is published: all users get notified
- When subscription is activated: user gets confirmed
- When subscription expires in 24 hours: warning notification
- When subscription expires: expiry notification

**Setup:** Requires a `firebase-service-account.json` file (downloaded from Firebase Console → Project Settings → Service Accounts).

---

### 6.3 Gmail SMTP (Email)

**What it does:** Sends transactional emails for OTP, payment receipts, subscription alerts.

**Why Gmail SMTP:**
- Free for low volume (<500 emails/day)
- Simple setup with App Password
- HTML email templates with professional formatting

**Emails sent:**

| Trigger | Email sent to | Template |
|---|---|---|
| Admin/User requests OTP | Admin or user | `otp.html` |
| New user registers | New user | `welcome.html` |
| Subscription activated | User | `subscription_confirm.html` |
| Payment completed | User | `payment_receipt.html` |
| Subscription expires in 24h | User | `expiry_warning.html` |
| Subscription expired | User | `subscription_expired.html` |
| Account suspended | User | `account_suspended.html` |
| New subscription/upgrade | Admin | `admin_sub_alert.html` |

All email sending is **asynchronous** (runs in a goroutine) so it never blocks or slows down the API response.

**Environment variables:**
```
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your@gmail.com
SMTP_PASS=your-app-password
SMTP_FROM=your@gmail.com
```

> For production at high volume, consider switching to SendGrid or AWS SES (both have free tiers).

---

### 6.4 Cloudflare R2 (File Storage)

**What it does:** Stores all uploaded videos and photos. Serves them via Cloudflare's global CDN.

**Why R2 instead of local disk:**
- **Zero egress fees** — no charge for video streaming bandwidth, no matter how many users watch
- **Global CDN** — video loads fast for users anywhere in India and worldwide
- **No server load** — after upload, the API never touches the video file again; streaming goes directly from R2 to the user's device
- **S3-compatible API** — works with the existing AWS SDK code, just needs a different endpoint

**Why not AWS S3:**
- AWS charges $0.09/GB for egress; a 100 MB video watched by 1000 users = $9 just for that one video
- R2 egress is free

**File organization in bucket:**
```
photos/{uuid}.jpg
avatars/{uuid}.jpg
community/{uuid}.jpg
videos/{uuid}.mp4   (videos)
videos/thumbnail/thumb_{uuid}.jpg (video thumbnails)
```

**Streaming flow:**
1. User taps "Play" in mobile app
2. App calls `GET /api/v1/user/videos/:id/stream-url`
3. API verifies subscription access, generates a **2-hour presigned URL** pointing to R2
4. API returns the URL in JSON response
5. Mobile app's video player streams directly from R2 using that URL
6. API is completely out of the video streaming loop

**Environment variables (hybrid — new uploads to R2, legacy files stay on disk):**
```
R2_BUCKET=your-bucket-name
R2_REGION=auto
R2_ACCESS_KEY=your-r2-access-key
R2_SECRET_KEY=your-r2-secret
R2_ENDPOINT=https://<ACCOUNT_ID>.r2.cloudflarestorage.com
R2_PUBLIC_BASE=https://pub-<hash>.r2.dev
```

When all `R2_*` fields are set, the API uses **hybrid** mode: new files go to R2; existing files on `./api/uploads/` keep working at `/uploads/*`. Set `STORAGE_LOCAL_ONLY=true` to force disk-only even with R2 creds present.

**Current setup:** Local disk until R2 credentials are configured. Files stored in `./api/uploads/`.

---

## 7. Database Tables

All tables use UUID primary keys (except `notification_logs` and `user_notifications` which use auto-increment for performance).

| Table | Purpose | Key fields |
|---|---|---|
| `admins` | Admin user accounts | `email` (unique), `first_name`, `last_name`, `is_active` |
| `users` | Student accounts | `email` (unique), `name`, `phone`, `status` (ACTIVE/SUSPENDED/EXPIRED), `avatar_url` |
| `otp_codes` | Admin OTP tokens | `admin_id`, `code_hash`, `expires_at`, `used` |
| `user_otp_codes` | User OTP / reset tokens | `user_id`, `code_hash`, `expires_at`, `used` |
| `refresh_tokens` | Admin long-lived tokens | `admin_id`, `token_hash`, `expires_at`, `revoked` |
| `user_refresh_tokens` | User long-lived tokens | `user_id`, `token_hash`, `expires_at`, `revoked` |
| `user_sessions` | Active user login sessions | `user_id`, `device_info`, `ip_address`, `jwt_jti`, `last_active_at`, `revoked_at` |
| `plans` | Subscription plans | `name`, `price_minor`, `duration_days`, `features`, `is_active` |
| `subscriptions` | User subscription records | `user_id`, `plan_id`, `status` (ACTIVE/EXPIRED/SUSPENDED/CANCELLED), `expiry_date` |
| `payments` | Payment transactions | `user_id`, `amount_minor`, `provider` (razorpay/stripe/MANUAL), `status`, `provider_payment_id` |
| `videos` | Video content | `title`, `category`, `status`, `file_key`, `thumbnail_url`, `is_free`, `is_preview` |
| `playback_logs` | Video view history | `user_id`, `video_id`, `watched_seconds`, `completed`, `played_at` |
| `video_progress` | Watch position per user/video | `user_id` + `video_id` (composite PK), `position_seconds` |
| `video_reactions` | Likes/dislikes | `user_id` + `video_id` (composite PK), `reaction` (like/dislike) |
| `video_comments` | Video comments | `video_id`, `user_id`, `content`, `created_at` |
| `photos` | Gallery photos | `title`, `category`, `file_key`, `is_published` |
| `community_posts` | Forum posts | `user_id`, `anon_name`, `category`, `title`, `content`, `reply_count`, `is_flagged` |
| `community_replies` | Forum replies | `post_id`, `user_id`, `content`, `is_flagged` |
| `device_tokens` | FCM push tokens | `user_id`, `token` (unique), `platform` (android/ios) |
| `notification_logs` | FCM broadcast history | `title`, `body`, `audience`, `sent_count`, `failed_count` |
| `user_notifications` | Per-user notification inbox | `user_id`, `notification_log_id`, `read` |
| `audit_logs` | Immutable event log | `event_type`, `actor_admin_id`, `actor_user_id`, `target_id`, `ip_address`, `details` (JSON) |

---

## 8. File Storage

### Local Disk (default, development)
Files are stored in `./api/uploads/` and served via Nginx/Fiber static middleware at `/uploads/*`.

```
./api/uploads/
├── photos/
│   └── {uuid}.jpg
├── avatars/
│   └── {uuid}.jpg
├── community/
│   └── {uuid}.jpg
└── videos/
    ├── {uuid}.mp4
    └── thumbnail/
        └── thumb_{uuid}.jpg
```

**Limits:**
- Photos: 10 MB per file
- Community images: 5 MB per file
- Videos: 4 GB per file (controlled by `MAX_FILE_SIZE_BYTES`)

### Cloudflare R2 (hybrid production)
When R2 credentials are configured, **new** uploads go to R2; **legacy** files remain on local disk and are served at `/uploads/*`. The same file key structure is used. Video streaming uses presigned R2 URLs (2-hour expiry) for objects stored in R2.

---

## 9. Background Jobs

### Subscription Expiry Checker
**File:** `api/internal/cron/subscription_checker.go`
**Schedule:** Every 24 hours (runs as a goroutine started at API startup)

**What it does each run:**
1. Finds all `ACTIVE` subscriptions expiring within 24 hours → sends expiry warning email
2. Finds all `ACTIVE` subscriptions past their `expiry_date` → marks as `EXPIRED`
3. Updates `users.status` to `EXPIRED` for users with no active subscription
4. Finds subscriptions expiring in 7 days → sends 7-day warning (if not already sent)

**Why this matters:** Without this job, expired users would still have access to paid videos. The job runs at startup and then every 24 hours.

---

## 10. Environment Variables

Complete reference for `api/.env`:

```env
# ── Server ────────────────────────────────────────────
PORT=3000                                # API listens on this port

# ── Database ──────────────────────────────────────────
DATABASE_URL=postgres://admin:password@localhost:5433/marketkit?sslmode=disable

# ── JWT Tokens ────────────────────────────────────────
JWT_SECRET=change-this-to-32-chars      # Must be strong random string in production
JWT_EXPIRY=8h                           # Access token lifetime

# ── Email (Gmail SMTP) ────────────────────────────────
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_SECURE=false
SMTP_USER=your@gmail.com
SMTP_PASS=your-gmail-app-password       # Gmail App Password (not your Gmail password)
SMTP_FROM=your@gmail.com

# ── Razorpay ──────────────────────────────────────────
RAZORPAY_KEY_ID=rzp_live_xxxx
RAZORPAY_KEY_SECRET=xxxx
RAZORPAY_WEBHOOK_SECRET=xxxx

# ── CORS ──────────────────────────────────────────────
CORS_ORIGIN=http://localhost:8080,...   # Comma-separated list of allowed origins

# ── File Uploads ──────────────────────────────────────
UPLOAD_DIR=./uploads                    # Local storage directory
MAX_FILE_SIZE_BYTES=4294967296          # 4 GB max video upload

# ── Redis Cache ───────────────────────────────────────
REDIS_URL=redis://localhost:6379        # Leave empty to disable caching

SERVER_BASE_URL=http://192.168.1.105:3000  # Used for legacy local file URLs

# ── Cloudflare R2 (optional — hybrid when all set) ───
R2_BUCKET=
R2_REGION=auto
R2_ACCESS_KEY=
R2_SECRET_KEY=
R2_ENDPOINT=                            # https://<ACCOUNT_ID>.r2.cloudflarestorage.com
R2_PUBLIC_BASE=                         # https://pub-<hash>.r2.dev or custom domain
# STORAGE_LOCAL_ONLY=true               # force disk-only

# ── PostgreSQL (Docker Compose only) ──────────────────
POSTGRES_USER=admin
POSTGRES_PASSWORD=password
POSTGRES_DB=marketkit

# ── Admin Seed Account ────────────────────────────────
ADMIN_EMAIL=your@gmail.com
ADMIN_NAME=Super Admin
```

---

## 11. API Endpoints Reference

### Authentication
| Method | Endpoint | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/auth/send-otp` | None | Send OTP to admin email |
| POST | `/api/v1/auth/verify-otp` | None | Login with OTP |
| POST | `/api/v1/auth/refresh` | None | Refresh admin JWT |
| POST | `/api/v1/auth/logout` | Admin | Logout |
| GET | `/api/v1/auth/me` | Admin | Get admin profile |
| POST | `/api/v1/user/auth/register` | None | Register student |
| POST | `/api/v1/user/auth/send-otp` | None | Send OTP to student email |
| POST | `/api/v1/user/auth/verify-otp` | None | Student login with OTP |
| POST | `/api/v1/user/auth/forgot-password` | None | Request password reset |
| POST | `/api/v1/user/auth/reset-password` | None | Reset password |
| POST | `/api/v1/user/auth/refresh` | None | Refresh user JWT |
| POST | `/api/v1/user/auth/logout` | User | Student logout |
| GET | `/api/v1/user/auth/me` | User | Get student profile |
| PATCH | `/api/v1/user/auth/me` | User | Update profile |
| POST | `/api/v1/user/auth/avatar` | User | Upload avatar |
| POST | `/api/v1/user/auth/change-password` | User | Change password |
| POST | `/api/v1/user/auth/device-token` | User | Register FCM token |

### Videos
| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/videos/` | Admin | List all videos |
| POST | `/api/v1/videos/create` | Admin | Upload video |
| GET | `/api/v1/videos/:id` | Admin | Get video details |
| PATCH | `/api/v1/videos/:id` | Admin | Update video |
| DELETE | `/api/v1/videos/:id` | Admin | Delete video |
| POST | `/api/v1/videos/:id/retry` | Admin | Retry failed video |
| GET | `/api/v1/videos/:id/stream` | Admin | Stream URL (redirect) |
| GET | `/api/v1/user/videos/` | User | List published videos |
| GET | `/api/v1/user/videos/:id/stream` | User | Stream URL (redirect) |
| GET | `/api/v1/user/videos/:id/stream-url` | User | Stream URL (JSON) |
| POST | `/api/v1/user/videos/:id/playback-log` | User | Log a video view |
| POST | `/api/v1/user/videos/:id/progress` | User | Save watch position |

### Photos
| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/photos/public` | None | Published photos |
| GET | `/api/v1/photos/` | Admin | All photos |
| POST | `/api/v1/photos/create` | Admin | Upload photo |
| PATCH | `/api/v1/photos/:id` | Admin | Update photo |
| DELETE | `/api/v1/photos/:id` | Admin | Delete photo |

### Plans & Payments
| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/plans/public` | None | Active plans list |
| GET | `/api/v1/plans/` | Admin | All plans |
| POST | `/api/v1/plans/create` | Admin | Create plan |
| PATCH | `/api/v1/plans/:id` | Admin | Update plan |
| DELETE | `/api/v1/plans/:id` | Admin | Delete plan |
| POST | `/api/v1/user/payments/order` | User | Create Razorpay order |
| POST | `/api/v1/payments/razorpay-webhook` | None (HMAC) | Razorpay webhook |
| GET | `/api/v1/payments/` | Admin | List payments |
| POST | `/api/v1/payments/manual` | Admin | Manual payment |
| POST | `/api/v1/payments/:id/activate` | Admin | Activate subscription |

### Users
| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/users/` | Admin | List users |
| POST | `/api/v1/users/create` | Admin | Create user |
| GET | `/api/v1/users/:id` | Admin | Get user |
| PATCH | `/api/v1/users/:id` | Admin | Update user |
| DELETE | `/api/v1/users/:id` | Admin | Delete user |
| POST | `/api/v1/users/:id/force-logout` | Admin | Force logout all sessions |
| PATCH | `/api/v1/users/:id/suspend` | Admin | Suspend/reinstate user |
| POST | `/api/v1/users/:id/change-plan` | Admin | Change subscription plan |

### Community, Notifications, Analytics
| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/user/community/posts` | User | Browse posts |
| POST | `/api/v1/user/community/posts` | User | Create post |
| POST | `/api/v1/user/community/posts/:id/replies` | User | Add reply |
| POST | `/api/v1/admin/notifications/broadcast` | Admin | Send push notification |
| GET | `/api/v1/dashboard/stats` | Admin | KPI stats |
| GET | `/api/v1/playback/top-videos` | Admin | Most watched videos |
| GET | `/api/v1/revenue/summary` | Admin | Revenue overview |
| GET | `/api/v1/audit-logs/` | Admin | Event log |
| GET | `/health` | None | Health check |
