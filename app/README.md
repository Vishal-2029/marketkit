# MarketKit — Flutter App

Mobile app for MarketKit. Users browse and buy products in the marketplace, manage a wallet, and stream subscription video content.

---

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Framework | Flutter 3.11+ |
| Language | Dart |
| State management | Riverpod 2 (StateNotifier, no codegen) |
| Navigation | GoRouter 14 (ShellRoute) |
| HTTP | Dio 5 with 401 interceptor |
| Video player | video_player + Chewie |
| Payments | razorpay_flutter |
| Storage | flutter_secure_storage |
| Images | cached_network_image |

---

## Setup

### Prerequisites

- Flutter 3.11+ (`flutter --version`)
- Android emulator or physical device (Android API 19+)

### Run

```bash
cd app
flutter pub get
flutter run
```

### Build

```bash
flutter build apk --release
flutter build ios --release
```

### Configure API URL

Edit `lib/core/network/api_endpoints.dart` and set `baseUrl` to your backend address:

```dart
static const String baseUrl = 'http://192.168.1.105:3000/api/v1';
```

Use your machine's LAN IP (not `localhost`) when running on a physical device or emulator.

---

## Project Structure

```
app/lib/
├── core/
│   ├── network/
│   │   ├── api_endpoints.dart      # All endpoint constants
│   │   └── dio_client.dart         # Singleton Dio + 401 interceptor
│   ├── router/
│   │   └── app_router.dart         # GoRouter config (all routes)
│   └── theme/
│       └── app_theme.dart          # Colors, fonts, ThemeData
├── features/
│   ├── auth/                       # OTP login, register, refresh
│   ├── home/                       # Home screen + video grid
│   ├── library/                    # Accessible videos list
│   ├── plans/                      # Subscription plan cards + checkout
│   ├── profile/                    # User profile + subscription status
│   └── video_player/               # Chewie video player screen
└── main.dart
```

Each feature follows the same layered structure:

```
features/<name>/
├── models/         # Data classes with fromJson/toJson
├── services/       # HTTP calls via DioClient
├── providers/      # Riverpod StateNotifier
├── screens/        # Full-page widgets
└── widgets/        # Reusable widgets within this feature
```

---

## Screens & Navigation

```
/                   SplashScreen        — checks stored token, redirects
/login              LoginScreen         — email input for OTP
/register           RegisterScreen      — new account form
/otp                OtpScreen           — 6-digit OTP entry
/video/:id          VideoPlayerScreen   — full-screen Chewie player (no bottom nav)

ShellRoute (bottom navigation bar):
  /home             HomeScreen          — featured + category video grid
  /library          LibraryScreen       — list of accessible videos
  /plans            PlansScreen         — subscription plan cards + Razorpay checkout
  /profile          ProfileScreen       — user info + subscription status + logout
```

The `/video/:id` route is a sibling of the ShellRoute so the bottom nav bar is hidden during playback. Navigate to it with:

```dart
context.push('/video/${video.id}', extra: video);
```

---

## State Management

All providers live under `features/<name>/providers/`. The main ones:

| Provider | State | Description |
|----------|-------|-------------|
| `authProvider` | `AuthState` | Current user, subscription, login/logout/refresh |
| `videosProvider` | `VideosState` | Video list with `accessible` flag per video |
| `plansProvider` | `PlansState` | Plan list, Razorpay order creation |

### Subscription state flow

```
App start / OTP verify
  → AuthService.tryRefresh() or verifyOtp()
  → GET /user/auth/me (returns user + active subscription)
  → AuthState.user populated → AuthState.hasSubscription computed

After payment:
  → PlansNotifier.refreshAfterPayment()
  → AuthNotifier.fetchMe() → re-fetches /user/auth/me
  → All watching widgets rebuild: home banner, plan cards, profile card
```

---

## Payment Flow

1. User taps "Buy Now" on a plan card
2. `PlansService.createOrder(planId)` calls `POST /user/payments/order`
3. Backend creates a Razorpay order and returns `{order_id, amount, key_id}`
4. `_razorpay.open({...})` launches native Razorpay checkout sheet
5. On `EVENT_PAYMENT_SUCCESS`, call `refreshAfterPayment()` to update subscription state
6. Razorpay fires `payment.captured` webhook to backend → backend marks payment success + creates subscription

---

## Color Palette

Defined in `lib/core/theme/app_theme.dart`:

| Constant | Role |
|----------|------|
| `kGold` | Primary / brand accent |
| `kCream` | Secondary / backgrounds |
| `kSage` | Success / active states |
| `kTerracotta` | Error / destructive actions |
| `kCard` | Card and bottom nav background |
| `kBackground` | Page background |

---

## Network Layer

`DioClient` is a singleton with:

- Base URL from `ApiEndpoints.baseUrl`
- `Authorization: Bearer <token>` header on every request
- `withCredentials: true` for httpOnly refresh token cookie
- 401 interceptor that automatically calls `POST /user/auth/refresh` and retries the original request once

Token is stored in `flutter_secure_storage` and loaded at app start.

---

## Video Streaming

`VideosService.streamUrl(id)` returns a URL in the form:

```
GET /user/videos/:id/stream?token=<jwt>
```

The `?token=` query parameter is used because `VideoPlayerController.networkUrl()` cannot set Authorization headers. The backend accepts the token in this query param as a fallback.

---

## Key Dependencies

```yaml
flutter_riverpod: ^2.5.1
go_router: ^14.2.0
dio: ^5.4.3
flutter_secure_storage: ^9.2.2
video_player: ^2.8.6
chewie: ^1.7.5
razorpay_flutter: ^1.3.7
google_fonts: ^6.2.1
cached_network_image: ^3.3.1
shimmer: ^3.0.0
```
