# Customize

Making MarketKit yours. Almost everything visual lives in one file per tier —
this page tells you which file, and what breaks if you change the wrong thing.

Work through it top to bottom the first time. Steps 1–4 take about an hour and
cover everything a buyer sees.

---

## 1. Name

| Where | File | Change |
|---|---|---|
| App (in-app text, checkout sheet) | `app/lib/core/config/brand.dart` | `Brand.name`, `supportEmail`, `privacyPolicyUrl` |
| Android app label | `app/android/app/src/main/AndroidManifest.xml` | `android:label` |
| iOS app name | `app/ios/Runner/Info.plist` | `CFBundleDisplayName`, `CFBundleName` |
| App window title | `app/lib/main.dart` | `title:` |
| Flutter web shell | `app/web/index.html`, `app/web/manifest.json` | `<title>`, `name`, `short_name` |
| Admin panel tab | `web/index.html` | `<title>` and the meta tags |
| Admin sidebar wordmark | `web/src/components/AdminLayout.tsx` | the wordmark `<span>` |
| Emails | `api/internal/email/templates/*.html` | 14 files — find and replace |
| API docs title | `api/cmd/api/main.go` | `@title`, `@contact.*`, then `make swagger` |

`Brand.checkoutName` is what buyers see inside the payment sheet at the moment
they pay. Keep it recognisable or people abandon the payment.

---

## 2. Bundle IDs

Do this **before** your first store submission — it cannot be changed
afterwards without publishing a new app.

Replace `com.example.marketkit` in:

```
app/android/app/build.gradle.kts          namespace + applicationId
app/android/app/proguard-rules.pro
app/android/app/src/main/kotlin/com/example/marketkit/MainActivity.kt
app/ios/Runner.xcodeproj/project.pbxproj  6 occurrences
app/linux/CMakeLists.txt
```

The Kotlin **directory path** must match the package too:

```bash
cd app/android/app/src/main/kotlin
mkdir -p com/yourcompany/yourapp
git mv com/example/marketkit/MainActivity.kt com/yourcompany/yourapp/
rm -rf com/example
# then update the `package` line inside MainActivity.kt
```

Leave `MainActivity` extending `FlutterFragmentActivity`. `flutter_stripe`'s
payment sheet is a Fragment — plain `FlutterActivity` compiles and then crashes
at runtime the first time someone pays.

The Dart package name is `marketkit` (`app/pubspec.yaml`). Renaming it means
updating every `package:marketkit/...` import, so it is optional — buyers never
see it.

The Go module is `github.com/marketkit/api`. To rename:

```bash
cd api
grep -rl 'github.com/marketkit/api' --include='*.go' . | \
  xargs sed -i 's|github.com/marketkit/api|github.com/you/yourapi|g'
sed -i '1s|.*|module github.com/you/yourapi|' go.mod
go mod tidy && go build ./...
```

---

## 3. Colours

Two files. Change the primary and everything else follows — the rest of the
palette is neutral and works with any hue.

**App** — `app/lib/core/theme/app_colors.dart`

```dart
const kPrimary      = Color(0xFF4F46E5);  // your brand colour
const kPrimaryLight = Color(0xFF818CF8);  // lighter, for gradients
```

**Admin panel** — `web/src/index.css`

```css
--primary: 243 75% 59%;       /* HSL channels, no hsl() wrapper */
--primary-glow: 234 89% 74%;
```

Tailwind composes these as `hsl(var(--primary) / alpha)`, so the values are bare
channels — wrapping them in `hsl()` breaks every colour on the page.

Keep the two in sync or the app and panel drift apart visually. A hex-to-HSL
converter is all you need.

Fonts for the panel are Google Fonts imports at the top of `web/src/index.css`
plus `fontFamily.sans` in `web/tailwind.config.ts`.

---

## 4. Logo

Two source files. Everything else is generated.

| File | Used by |
|---|---|
| `app/assets/icon.png` | Flutter launcher icons — square, 1024×1024 |
| `web/public/icon.svg` | Admin panel sidebar, login screen, favicon |

After replacing the PNG, regenerate every platform icon:

```bash
cd app
dart run flutter_launcher_icons
```

That rewrites the Android mipmaps, the iOS AppIcon set, the web icons and the
favicon from your single source image.

`web/public/icon-email-bg.jpg` is the background strip in transactional emails.

---

## 5. Categories and plan features

Content categories double as plan feature keys: a plan grants access to a
category by listing its key, and a video unlocks when its category appears in
the viewer's feature set. One list per tier — keep all three in step:

| Tier | File |
|---|---|
| Go | `api/internal/models/video.go` — `CategoryA/B/C` + `AllVideoCategories` |
| Flutter | `app/lib/core/config/feature_catalog.dart` |
| React | `web/src/lib/featureCatalog.ts` |

The shipped keys are placeholders (`CATEGORY_A`, `CATEGORY_B`, `CATEGORY_C`).
Rename them to your own taxonomy — nothing else hardcodes those values.

Keys are free-form strings, and a plan's `features` is a list, so à-la-carte
plans ("A only", "A + B") and simple tiers both work with no schema change.
Entitlements are the **union** across a user's active subscriptions.

Change the keys **before** you have live subscribers — existing plan rows store
the old keys and will stop matching.

---

## 6. Currency

One deployment, one currency. Set it in `api/.env`:

```env
PAYMENT_CURRENCY=USD
```

Then match the clients:

```bash
# Admin panel
echo 'VITE_CURRENCY=USD' >> web/.env

# App — at build time
flutter run --dart-define=CURRENCY=USD
```

All three know the real ISO-4217 exponents, so JPY (0 decimals) and KWD (3) are
handled correctly. **Amounts are stored in the currency's minor unit** — cents
for USD, paise for INR. Changing the currency does not convert existing rows, so
decide before you take the first payment.

See [WALLET.md](WALLET.md) for why amounts are integers.

---

## 7. Payment gateway

```env
PAYMENT_PROVIDER=razorpay     # or: stripe
```

Then fill that gateway's keys in `api/.env`. Both are implemented; the app picks
the right checkout sheet from the API response, so no client change is needed.

**Adding a third gateway** means one file implementing five methods:

```go
// internal/payments/provider/provider.go
Name() / CreateOrder() / VerifyCheckout() / ParseWebhook() / Refund()
```

Register it in `internal/payments/payments.go` and it becomes selectable. No
module that takes money needs to change — see
`internal/payments/provider/stripe/` as a worked example.

Two rules for any implementation:

- `ParseWebhook` must **fail closed**. An unconfigured signing secret is an
  error, never a pass — with an empty key the signature is trivially forgeable.
- If the gateway has no client-side signature, `VerifyCheckout` must confirm
  with the gateway rather than trusting the client. Stripe's implementation
  shows the pattern.

---

## 8. What sellers can upload

`api/internal/modules/market/handler.go` → `productFileExts`.

The default set covers common digital goods: documents, images, design source
files, 3D, fonts, audio/video, archives. This map is the only place upload types
are enforced — add or remove extensions to match what you sell.

Size limits are the constants just below it (`maxPreviewImageBytes`,
`minPriceMinor`, `maxPriceMinor`). The price bounds are in minor units and are
mirrored in `app/lib/features/marketplace/screens/upload_product_tab.dart` —
change both or the form accepts what the server rejects.

---

## 9. Removing what you don't need

The kit ships the full product: a marketplace **and** a video/learning side with
community and photos. If you only want the marketplace, these are safe to
remove.

| Module | Go | Flutter | React |
|---|---|---|---|
| Videos | `modules/{videos,user_videos,playback,video_engagement}` | `features/{home,video_player,downloads,library}` | `pages/VideosPage.tsx` |
| Playlists | `modules/{playlists,user_playlists}` | — | `pages/PlaylistsPage.tsx` |
| Community | `modules/community` | `features/community` | `pages/CommunityPage.tsx` |
| Photos | `modules/{photos,photo_categories}` | `features/photos` | `pages/PhotosPage.tsx` |
| Learning plans | `modules/plans`, `internal/subscriptions` | `features/plans` | `pages/PlansPage.tsx` |

How to do it safely:

1. Delete the module directory
2. Remove its routes from `api/internal/routes.go`
3. Remove its models from the `AutoMigrate` list in `internal/database/database.go`
4. Remove the Flutter routes from `app/lib/core/router/app_router.dart` and the
   React routes from `web/src/App.tsx`
5. `go build ./...` — the compiler finds the rest

Do it one module at a time and build in between. `routes.go` registers every
module in one file, so that is where most of the edits land.

Removing learning plans also removes the `plans` entitlement model, so drop
`Plan.Features` handling from `subscriptions` too.

---

## 10. Before you ship

- [ ] Bundle IDs changed (permanent after store submission)
- [ ] Logo replaced and launcher icons regenerated
- [ ] Colours updated in both files
- [ ] Category keys renamed, all three tiers matching
- [ ] Currency set, clients matching
- [ ] Gateway keys in `api/.env`, webhook URL registered
- [ ] `make swagger` re-run after editing API annotations
- [ ] Email templates rebranded
- [ ] `.env` files are **not** committed — check `git status`

Then [DEPLOY.md](DEPLOY.md).
