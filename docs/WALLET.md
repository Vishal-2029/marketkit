# The wallet ledger

This is the part of MarketKit that takes the longest to build correctly, and the
part most worth understanding before you change anything. Read this before
touching `internal/modules/wallet` or `internal/modules/platform_wallet`.

---

## The shape of it

There are **two** independent ledgers.

| | Who owns it | Table | Balance lives on |
|---|---|---|---|
| **User wallet** | every user | `wallet_transactions` | `users.wallet_balance_minor` |
| **Platform wallet** | your business | `platform_ledgers` | `platform_wallets.balance_minor` |

Both are **append-only**. Rows are never updated or deleted. Every row carries a
`balance_after_minor` snapshot, so you can reconstruct the balance at any point
in history without replaying from zero.

---

## Two invariants

Everything else is detail. These two must always hold:

```
per user:   SUM(wallet_transactions.amount_minor)  ==  users.wallet_balance_minor
globally:   SUM(platform_ledgers.amount_minor)     ==  platform_wallets.balance_minor
```

The cached balance column exists only so you don't have to `SUM()` on every
read. The ledger is the source of truth.

Check them at any time:

```sql
SELECT u.id, u.wallet_balance_minor, COALESCE(SUM(wt.amount_minor), 0) AS ledger
FROM users u
LEFT JOIN wallet_transactions wt ON wt.user_id = u.id::text
GROUP BY u.id, u.wallet_balance_minor
HAVING u.wallet_balance_minor <> COALESCE(SUM(wt.amount_minor), 0);
```

Zero rows means healthy. `api/seed/demo.go` runs exactly this check after
seeding and refuses to report success if it fails.

> `users.id` is `uuid` while `wallet_transactions.user_id` is `text`, hence the
> `::text` cast. Without it Postgres raises
> `operator does not exist: text = uuid`.

---

## Money is always integer minor units

Every monetary column is an `int64` in the currency's **minor unit** — paise for
INR, cents for USD, yen for JPY (which has no minor unit at all).

Never use floats for money. `0.1 + 0.2 != 0.3` in binary floating point, and a
platform fee split that drifts by a fraction of a unit per sale becomes a real
accounting problem at volume. Integers make the fee and the seller's net add
back to the price *exactly*, every time.

Formatting for display is the only place the number becomes decimal:

| Tier | Helper |
|---|---|
| Go | `pkg/money` — `money.Format(499900, "INR")` → `₹4,999` |
| Flutter | `core/config/currency.dart` — `Currency.format(499900)` |
| React | `lib/currency.ts` — `formatMoney(499900)` |

All three know the real ISO-4217 exponents, so they handle JPY (0 decimals) and
KWD (3 decimals) correctly. Dividing by 100 inline is wrong for those.

---

## The one function that moves user money

```go
wallet.Apply(tx, userID, txType, amountMinor, referenceID, meta) (newBalance, error)
```

Credits are positive, debits negative. In one call it:

1. `SELECT … FOR UPDATE` on the user row — concurrent debits serialize
2. Refuses to go below zero (`ErrInsufficientBalance`)
3. Updates the cached balance
4. Appends the ledger row with the new balance snapshot

**It must be called inside a transaction you own.** That is what makes the
balance change and the ledger row atomic — a rollback undoes both, so the two
can never disagree.

Transaction types (`models.WalletTx*`):

| Type | Sign | When |
|---|---|---|
| `TOPUP` | + | User adds money via a payment gateway |
| `SALE_CREDIT` | + | A seller's product sold (net of fee) |
| `PURCHASE_DEBIT` | − | Buyer paid for a product from wallet |
| `PLAN_DEBIT` | − | Seller paid for a market plan from wallet |
| `WITHDRAWAL` | − | Seller cashed out |

### Lock ordering

When one transaction touches two wallets, **always debit the buyer before
crediting the seller.** A single fixed order across the whole codebase is what
prevents deadlocks: two concurrent purchases can never each hold the lock the
other wants.

If you add a flow that touches two wallets, keep this order.

---

## A sale, end to end

Buyer purchases a ₹999.00 product from their wallet. Platform fee is 10%.

```
price  = 99900 minor units
fee    = 99900 * 10 / 100 = 9990      (floored — integer division)
net    = 99900 - 9990     = 89910
```

All of this happens in **one** transaction (`market/wallet_purchase.go`):

| Step | Ledger | Amount |
|---|---|---|
| 1. Create the purchase row | — | — |
| 2. Debit buyer | user wallet | `−99900` `PURCHASE_DEBIT` |
| 3. Increment `products.sales_count` | — | — |
| 4. Credit seller | user wallet | `+89910` `SALE_CREDIT` |
| 5. Credit platform | platform wallet | `+9990` `PLATFORM_FEE` |

`9990 + 89910 == 99900`. The money is conserved because it is all integers.

If any step fails, the whole transaction rolls back — no half-completed sale.

The fee percentage is admin-tunable (`wallet.FeePercent()`, default 10) and is
**snapshotted onto the purchase row** as `fee_minor` / `seller_net_minor`. A
historical sale keeps the fee it was sold under even after you change the rate.

---

## The platform wallet

One row, fixed id `"platform"`. It is your business's running income, not a
per-admin balance, and only super-admins can see or withdraw from it.

Credit sources (`models.PlatformSource*`):

| Source | From |
|---|---|
| `PLATFORM_FEE` | Your cut of each product sale |
| `MARKET_PLAN` | Sellers subscribing to a market plan |
| `LEARNING_PLAN` | Learning subscription payments |
| `WITHDRAWAL` | Super-admin cashing out (the only debit) |

`platform_wallet.Apply` mirrors `wallet.Apply` exactly — same row lock, same
append-only ledger, same must-be-in-a-transaction rule.

### `Backfill()`

Runs once at boot, guarded by `platform_wallets.backfilled_at`, and seeds the
platform balance from historical rows. It exists because the platform wallet was
added to an app that already had sales.

**On a fresh install it has nothing to do.** If you are starting clean you can
safely delete the call from `cmd/api/main.go`.

---

## Top-ups and withdrawals

**Top-up** — user pays a gateway, money lands in their wallet. The gateway
webhook is authoritative: even if the in-app confirmation fails, the webhook
still credits the wallet. Both paths are idempotent, so a double callback
cannot double-credit.

**Withdrawal** — a seller requests a payout.

- Minimum is admin-tunable (`wallet.MinWithdrawal()`, default `10000` minor units)
- The balance is **debited immediately** on request, not on settlement. There is
  no approval gate by design: the money leaves the wallet so it cannot be spent
  twice while the payout is in flight.
- An admin then marks it settled after sending the money out-of-band

The kit does **not** integrate a bank payout API. Withdrawals record intent and
capture payout details (UPI or bank); actually sending the money is manual, or
your own integration. That is a deliberate boundary — payout APIs are
country-specific.

---

## Refunds

Refunds go through the payment gateway (`refunds/handler.go`), not the wallet.
Manual (offline) payments cannot be refunded this way; anything a gateway took,
that gateway can refund.

### A known reconciliation gap

Read this before going live. There is a window where the gateway has issued a
refund but the database write fails:

```
1. Call the gateway's refund API      → succeeds
2. Mark the request APPROVED,
   cancel the subscription            → fails (DB error)
```

The money is gone from your account but the app does not know it. The code
handles this as well as it can without distributed transactions: it logs at
ERROR level with the refund id, user, and amount, stores the `refund_id` on the
request anyway, and tells the caller to reconcile manually.

```
refund: gateway refund SUCCEEDED but DB update failed — manual reconciliation needed
```

**Alert on that log line.** It is rare, but it is real money, and it needs a
human.

---

## If you change this code

- Never write `wallet_balance_minor` or `balance_minor` directly. Go through
  `Apply`, or the invariants break silently.
- Never update or delete a ledger row. Append a compensating entry instead.
- Keep every money mutation inside a transaction that also covers whatever
  caused it.
- Keep the buyer-before-seller lock order.
- Keep arithmetic in integer minor units. Convert to decimal only for display.

The tests worth reading first:

```
internal/modules/platform_wallet/ledger_test.go   ledger mechanics + invariants
internal/modules/market/wallet_purchase_test.go   the full fee-split path
pkg/money/money_test.go                           why integers, formatting rules
```

Run them with `make test`.
