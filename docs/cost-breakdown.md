# Stitch Craft Learn — Live System Cost Breakdown

Full cost for a completely live system including:
- Go API + PostgreSQL (backend)
- React Admin Panel (web)
- Flutter App (Android + iOS)
- 500GB video storage
- 300+ active users

---

## 1. Server — ₹700/month

**What it is:**
A machine running 24/7 on the internet that hosts your Go API and PostgreSQL database. All app logic runs here — user login, video access, subscription management, payments, push notifications.

**Why you pay:**
Without a server, your app has no backend. No backend means no login, no videos, no users, nothing works. This machine must stay ON 24/7 for all 300+ users to access the app at any time.

| | Cost |
|--|------|
| Monthly | ₹700 |
| Yearly | ₹8,400 |

---

## 2. Domain Name — ₹105/month

**What it is:**
Your internet address e.g. `stitchcraftlearn.com`. This is what your Flutter app and admin panel use to connect to the server. SSL certificate (HTTPS) is included free.

**Why you pay:**
Without a domain, your app has no fixed address to connect to. The Flutter app and admin panel both need a stable URL to reach the API. A domain gives you that permanent address.

| | Cost |
|--|------|
| Monthly | ₹105 |
| Yearly | ₹1,260 |

---

## 3. Storage (Cloudflare R2, 500GB) — ₹630/month

**What it is:**
Cloud space where all your video files, images, and uploaded content are stored. Users stream videos directly from here.

**Why you pay:**
Video files are large. The server's built-in 80GB disk fills up quickly. 500GB of dedicated cloud storage keeps all your videos safe, always available, and fast to stream — even if 50 users are watching at the same time.

| | Cost |
|--|------|
| Monthly | ₹630 |
| Yearly | ₹7,560 |

---

## 4. React Admin Panel — ₹0/month

**What it is:**
The web dashboard where you manage users, videos, plans, payments, and reports.

**Why free:**
The admin panel is a static website (HTML + CSS + JS). Vercel hosts static websites permanently for free with no limits on traffic.

| | Cost |
|--|------|
| Monthly | ₹0 |
| Yearly | ₹0 |

---

## 5. Google Play Store — ₹2,100 (one time only)

**What it is:**
Registration fee to publish your Flutter app on Android (Google Play Store).

**Why you pay:**
Google requires a one-time payment to verify you as a developer. After this, you publish unlimited app updates forever at no extra cost.

| | Cost |
|--|------|
| One-time | ₹2,100 |
| Every year after | ₹0 |

---

## 6. Apple App Store — ₹692/month

**What it is:**
Annual fee to publish your Flutter app on iPhone and iPad (Apple App Store).

**Why you pay:**
Apple charges every year to keep your app live on the App Store. If you stop paying, your iOS app is removed and iPhone users cannot download it.

| | Cost |
|--|------|
| Monthly | ₹692 |
| Yearly | ₹8,300 |

---

## 7. Firebase Push Notifications — ₹0/month

**What it is:**
Service that sends push notifications to users' phones (e.g. new video uploaded, subscription expiring).

**Why free:**
Firebase Cloud Messaging (FCM) is completely free with no usage limits. Google provides this at no cost.

| | Cost |
|--|------|
| Monthly | ₹0 |
| Yearly | ₹0 |

---

## Final Total

| What | Monthly | Yearly |
|------|---------|--------|
| Server (Hetzner CX32) | ₹700 | ₹8,400 |
| Domain (.com) | ₹105 | ₹1,260 |
| Storage (Cloudflare R2, 500GB) | ₹630 | ₹7,560 |
| Admin Panel (Vercel) | ₹0 | ₹0 |
| Android — Google Play Store | ₹0 | ₹0 |
| iOS — Apple App Store | ₹692 | ₹8,300 |
| Push Notifications (Firebase) | ₹0 | ₹0 |
| **Total** | **₹2,127/month** | **₹25,520/year** |

**First year only — add ₹2,100 (Play Store one-time registration) = ₹27,620**

---

> Note: Razorpay (payment gateway) has no fixed monthly cost. It charges 2% per transaction automatically from each payment received.
