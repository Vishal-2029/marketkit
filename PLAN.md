# MarketKit — Plan

**Started:** 25 July 2026
**Owner:** Vishal Sabhadiya

---

## 1. What is this product?

A **starter code kit** that other developers buy.

They want to build a marketplace app — where users buy and sell, money goes in a
wallet, sellers take their money out, and the platform takes a fee.

Building the money part takes them 3–6 months. Many give up.

They buy this kit, change the name and colors, and launch in one week.

**They pay one time. No monthly fee.**

---

## 2. Why people will buy it

Today you can buy website starter code easily (ShipFast and many others).

But:

- Flutter kits give you **Firebase** — not a real backend you own
- Go kits are **web only** — no mobile app
- **Every kit is "SaaS with Stripe subscription"** — none handle money moving
  between users

Nobody sells **Go backend + Flutter app + React admin panel together**.
Nobody sells **wallet + seller payouts + platform fee**.

That is the gap. That is what we sell.

Market size: SaaS boilerplate market crossed **$50M/year in 2026**.

---

## 3. What is inside the kit

Three folders in one zip:

| Folder | What |
|---|---|
| `api/` | Go server (Fiber + GORM + PostgreSQL) |
| `app/` | Flutter app — Android + iOS |
| `web/` | React admin panel |

Plus Docker, database migrations, seed data, and full documents.

### Core modules (in the kit)

```
auth · user_auth · sessions · users · admins
payments · wallet · platform_wallet
refunds · revenue · market · plans
notifications · audit_logs · dashboard
```

### Add-on packs (sell separately, later)

```
videos · playback · playlists · community · photos
```

Same buyer pays two or three times. This is important.

---

## 4. What must change from the old code

| Now | Change to |
|---|---|
| "Design Express" name and logo | Generic / empty |
| Embroidery, Willcom, E4, meCAD words | Generic "product" and "category" |
| Razorpay only | **Stripe + Razorpay** (Stripe = world) |
| Real Firebase and R2 keys | Empty `.env.example` |
| Real data | Fake demo data (seed file) |
| Few documents | Full setup guide + video |

The code stays. Only the **client-specific parts** go out.

---

## 5. Price

| Package | Price | What |
|---|---|---|
| Starter | **$299** | Code for 1 app |
| Pro | **$499** | Unlimited apps + 1 year updates |
| Add-on: Video pack | **$149** | Video / HLS system |
| Add-on: Community pack | **$99** | Posts, comments, reactions |

Take payment with **Lemon Squeezy** or **Paddle**.
They handle world tax. (Not Razorpay — that is India only.)

**Updates renew at ~30% per year.** Decide this before launch, not after.

**Target:** 100 sales in year 1 = ~$35,000. 500 sales = ~$175,000.

---

## 6. Five-week plan

### Week 1 — Clean
- [ ] Copy code into this new folder
- [ ] Remove all Design Express names, logos, colors
- [ ] Remove embroidery words → generic words
- [ ] Remove all real keys → `.env.example`

### Week 2 — Stripe + demo data
- [ ] Add Stripe next to Razorpay
- [ ] Seed file: 20 fake products, 5 fake sellers, fake transactions
- [ ] One command starts everything (`docker compose up`)

### Week 3 — Documents (50% of the product)
- [ ] `docs/INSTALL.md` — install in 10 minutes
- [ ] `docs/WALLET.md` — how the wallet ledger works
- [ ] `docs/CUSTOMIZE.md` — change name, colors, logo
- [ ] `docs/DEPLOY.md` — put it live

> Bad documents = refunds. Good documents = 5-star reviews.

### Week 4 — Demo + website
- [ ] Live demo online — anyone can open and click
- [ ] One-page website in `site/` — screenshots, features, price, Buy button
- [ ] Connect Lemon Squeezy / Paddle

### Week 5 — Launch
- [ ] Reddit: r/FlutterDev, r/golang, r/SideProject
- [ ] Free listing: starterindex.com, boilerplatelist.com, saasboilerplates.dev
- [ ] Product Hunt
- [ ] YouTube: "Build a marketplace app in 1 hour"

---

## 7. The free advertisement

Put **only the wallet ledger code** on GitHub — free, open source.

Developers search "how to build wallet ledger". They find it.
They see the code is good. They trust you. Then they buy the full kit.

Free code works while you sleep.

---

## 8. Rules to remember

1. **Documents are the product.** Code without documents gets refunded.
2. **The demo is the sales pitch.** Nobody buys a kit they cannot try.
3. **Stay narrow.** This is a *marketplace* kit, not "everything" kit.
4. **Answer buyers fast.** Early reviews decide everything.
5. **Never ship real keys.** Check `.env.example` before every release.

---

## 9. Folder map

```
marketkit/
├── PLAN.md          <- this file
├── api/             Go backend
│   ├── cmd/server/
│   ├── internal/
│   │   ├── config/ database/ middleware/ models/
│   │   ├── storage/ email/ testutil/
│   │   └── modules/   (auth, wallet, market, payments, ...)
│   ├── migrations/
│   └── seed/
├── app/             Flutter app
│   └── lib/
│       ├── core/ shared/
│       └── features/  (auth, marketplace, wallet, payouts, ...)
├── web/             React admin panel
│   └── src/
│       └── components/ pages/ hooks/ services/ lib/ contexts/
├── docs/            buyer documents
├── site/            landing page
└── scripts/         helper scripts
```

---

## 10. Next step

**Week 1, task 1** — start moving code in and removing client parts.
