# Testing

Three layers, 88 checks. Each one exists because something specific can go
wrong — this page says what, so you know which failure you are looking at when
something goes red.

| Layer | Command | Count | Needs |
|---|---|---|---|
| Go unit + integration | `make test` | 56 | Docker (throwaway Postgres) |
| End-to-end + attacks | `make smoke` | 25 | Running stack + demo data |
| Flutter | `cd app && flutter test` | 7 | Flutter SDK |

```bash
make test                       # backend, isolated database, no stack needed
make quickstart && make smoke   # full stack, real HTTP, real money path
```

---

## Why these tests exist

The kit's value is the money layer. Money bugs are the expensive kind: they are
silent, they compound, and you find them in an accounting mismatch weeks later
rather than in a stack trace. So the tests concentrate where a mistake costs
real money or leaks paid content, and stay thin everywhere else.

Three rules shaped what got a test:

1. **Anything that moves money** gets one. Balances must reconcile exactly.
2. **Anything that can silently pass** gets one — a check that cannot fail is
   worse than no check, because it reads as reassurance.
3. **Anything an attacker touches without logging in** gets one.

CRUD handlers that only read a row and return it are deliberately untested.
They fail loudly, and a test there costs maintenance without buying safety.

---

## `make test` — backend

Runs against a real throwaway PostgreSQL on port 5434, torn down afterwards,
never your dev database. Postgres and not SQLite because the ledger depends on
Postgres-only behaviour: `SELECT … FOR UPDATE` row locking, `jsonb`, and
`gen_random_uuid()`.

### The money layer

| File | Why |
|---|---|
| `platform_wallet/ledger_test.go` | Asserts the core invariant: wallet balance always equals `SUM(ledger rows)`. If this breaks, every revenue number in the product is wrong. |
| `market/wallet_purchase_test.go` | One sale debits the buyer, credits the seller, credits the platform — in one transaction. Checks the three amounts sum back to the price and that a rollback leaves nothing behind. |
| `market/purchase_test.go` | Purchase capture is idempotent. Gateways retry webhooks; a double callback must not pay a seller twice. |
| `market/revenue_test.go`, `revenue/handler_test.go` | Marketplace and learning revenue are calculated separately and must not contaminate each other. |
| `market/invoice_test.go` | Invoice totals match the stored purchase, including the fee snapshot. |
| `pkg/money/money_test.go` | Integer formatting across currencies. Caught a real bug: trailing-zero trimming rendered `99.90` as `99.9`. Also proves a fee split adds back to the price exactly — the reason amounts are integers. |

### Payments

| File | Why |
|---|---|
| `provider/razorpay/…`, `provider/stripe/…` | Weighted toward signature verification, because the webhook is the only unauthenticated write path into the money layer. Both providers are tested for: unconfigured secret **fails closed**, forged signature rejected, body tampered after signing rejected, unrelated events ignored rather than erroring. Stripe additionally covers the replay window — a captured webhook must stop working once its timestamp ages out. |
| `payments/capture/capture_test.go` | Dispatch stops at the first handler that claims an order. If two claimed the same payment, one module would process another's money. |
| `payments/handler_test.go`, `user_payments/handler_test.go` | Webhook capture end to end: status flip, platform credit, subscription activation. |

### Entitlements and access

| File | Why |
|---|---|
| `subscriptions/subscriptions_test.go` | Plan features round-trip through GORM's JSON serializer. **This caught a real bug**: a map-based `Updates()` bypasses the serializer and writes Go slice syntax, so `PATCH /plans/:id` would have silently corrupted the column. Also covers union-across-subscriptions and that expired plans grant nothing. |
| `storage/signing_test.go` | Download links must be signed, expiring, and bound to one file. **This covers a real vulnerability**: product files were previously served unauthenticated from `/uploads`, making paid content free to anyone with the URL. Asserts unsigned, forged, expired, and wrong-file links are all refused, and that responses force `attachment` so an uploaded SVG cannot execute on the API's origin. |

---

## `make smoke` — end to end

`scripts/smoke.sh`. Drives the real HTTP API the way a client does: logs in with
an emailed OTP read out of Mailhog, buys something, then attacks itself.

Unit tests prove functions behave. This proves the *system* behaves — routing,
middleware order, auth, and serialization all participate, and those are exactly
what unit tests cannot see.

```bash
make quickstart      # stack + demo data
make smoke
```

### What it walks through

| Section | Why |
|---|---|
| **Health** | The stack is actually up before anything else is believed. |
| **Login** | Full two-step flow: email + password, then the emailed code. Exercises OTP generation, delivery, verification, and JWT issue. |
| **Browse** | Product listing returns seeded data — proves the read path and demo seed together. |
| **Wallet** | Balance is readable and matches the ledger. |
| **Purchase** | The one that matters. Buys the cheapest product the buyer does not already own, then asserts the buyer was debited **exactly** the price — not approximately. |
| **Download** | The signed link resolves for the buyer, **and the same URL with its signature stripped is refused**. Both halves are the test: one proves the feature works, the other proves it is not free. |
| **Ledger invariant** | Re-checks balance = `SUM(transactions)` for every user *after* the purchase, against the live database. |

### The attack half

Each of these maps to a way marketplaces actually get robbed.

| Check | The attack it blocks |
|---|---|
| **Unauthenticated access** | Endpoints reachable with no token at all. |
| **Privilege escalation** | A normal user token on admin routes. Roles are read from the database, not the JWT, so a forged token cannot escalate. |
| **IDOR** | Changing an ID to read someone else's invoice — the classic marketplace data leak. |
| **Business logic** | Negative top-ups (paying yourself), oversized amounts, negative withdrawals. Logic flaws, not code bugs; a type system will not catch them. |
| **Internal exposure** | `.env`, `.git`, and path traversal served over HTTP. |
| **Injection** | SQL metacharacters in search treated as data, not code. |

Re-run `make smoke` after any change to auth, payments, or uploads. New code is
new attack surface.

---

## Flutter

```bash
cd app && flutter test
```

Seven tests over app-mode resolution and the category filter flow — the two
places where wrong state silently shows the user the wrong content. Payment
sheets are not covered: they need a real device and live gateway keys.

---

## What is not tested

Stated plainly so you know where you are on your own:

- **No real payment has run end to end.** Both gateways are tested against mock
  servers. Before launch, complete one test-mode payment per gateway on a real
  device.
- **Push notifications** need a real Firebase project.
- **Email delivery** is verified into Mailhog, not a real provider.
- **The admin panel and mobile UI have no automated tests.** `tsc` and
  `flutter analyze` catch type errors; nothing catches a broken layout.
- **No load testing.** The row locks are correct under concurrency by design and
  by the fixed lock ordering, but nobody has thrown a thousand concurrent
  purchases at them.

---

## Adding tests

Follow the same rule the existing ones follow: write a test when the failure
would be **silent, expensive, or reachable by a stranger**. Skip it when the
failure is obvious and cheap.

If you add a payment provider, copy `provider/stripe/stripe_test.go` — it is the
template for the checks a gateway must pass, and the fail-closed and replay
cases are the ones that matter.
