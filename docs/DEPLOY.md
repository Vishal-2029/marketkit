# Deploy

Getting MarketKit onto a server. This covers a single VPS running the whole
stack in Docker, which is what most buyers want and what the compose files are
written for.

---

## What you need

- A VPS with 2 GB RAM minimum (4 GB if you transcode video), Docker + Compose v2
- A domain pointed at the server's IP
- Ports 80 and 443 open

The API, Postgres, Redis and nginx all run on one box. Postgres data lives in a
named Docker volume that survives container rebuilds.

---

## 1. Get the code onto the server

```bash
ssh you@your-server
git clone <your-repo> marketkit && cd marketkit
```

## 2. Create the two env files

Production does **not** use `make bootstrap` — you want to control these values.

**Root `.env`** — read by Docker Compose itself:

```bash
cp .env.example .env
sed -i "s/changeme_use_a_strong_random_value/$(openssl rand -hex 24)/" .env
```

**`api/.env`** — read by the API process:

```bash
cp api/.env.live.example api/.env
```

Fill in, at minimum:

```env
APP_ENV=production
DATABASE_URL=postgres://marketkit:<same password as root .env>@postgres:5432/marketkit?sslmode=disable
JWT_SECRET=<openssl rand -hex 32>
ADMIN_EMAIL=you@yourdomain.com
ADMIN_PASSWORD=<strong password — change it after first login>

SERVER_BASE_URL=https://yourdomain.com
CORS_ORIGIN=https://yourdomain.com
FRONTEND_URL=https://yourdomain.com

PAYMENT_PROVIDER=razorpay
PAYMENT_CURRENCY=INR

SMTP_HOST=…
SMTP_PORT=587
SMTP_USER=…
SMTP_PASS=…
SMTP_FROM=noreply@yourdomain.com
```

The Postgres password must be **identical** in both files. Compose reads the
root `.env`; the API reads `api/.env`; nothing reconciles them for you.

`scripts/deploy.sh` refuses to run if the root `.env` is missing or still holds
the placeholder password. That guard exists because the fallback would silently
be `admin/password`.

## 3. TLS

nginx expects Let's Encrypt certificates:

```bash
sudo apt install certbot
sudo certbot certonly --standalone -d yourdomain.com -d www.yourdomain.com
```

Then set your domain in `nginx/nginx.conf` — replace `example.com` in the
`server_name` lines and the two `ssl_certificate` paths.

## 4. Deploy

```bash
make deploy
```

That builds the admin panel, then starts the production stack detached. Check:

```bash
docker compose ps
curl https://yourdomain.com/health
```

## 5. Create the first admin

```bash
docker compose exec api go run /app/seed.go
```

Uses the `ADMIN_*` values from `api/.env`. Log in and change the password.

> Do **not** run `seed/demo.go` in production — it creates fake sellers,
> products and purchases.

---

## Payment webhooks

Webhooks are how money actually gets confirmed. Without them, payments succeed
at the gateway and your database never finds out.

Register this URL in your gateway dashboard:

```
https://yourdomain.com/api/v1/payments/webhook
```

| Gateway | Event | Secret goes in |
|---|---|---|
| Razorpay | `payment.captured` | `RAZORPAY_WEBHOOK_SECRET` |
| Stripe | `payment_intent.succeeded` | `STRIPE_WEBHOOK_SECRET` |

`/api/v1/payments/razorpay-webhook` still works as a legacy alias.

**The endpoint fails closed.** If the signing secret is empty it rejects every
request rather than accepting unsigned ones. An empty HMAC key is trivially
forgeable, so "not configured" must never mean "allowed".

Test before trusting it:

```bash
# Razorpay: dashboard → Webhooks → Send test webhook
# Stripe:
stripe listen --forward-to https://yourdomain.com/api/v1/payments/webhook
stripe trigger payment_intent.succeeded
```

Then confirm it arrived:

```bash
docker compose logs api | grep -i webhook
```

Stripe additionally rejects webhooks whose timestamp is more than five minutes
old, which stops a captured request being replayed. If your server clock drifts,
legitimate webhooks start failing — keep NTP running.

---

## Mobile app

```bash
cd app
flutter build apk --release \
  --dart-define=API_BASE_URL=https://yourdomain.com/api/v1 \
  --dart-define=CURRENCY=INR
```

Output: `build/app/outputs/flutter-apk/app-release.apk`

For iOS, `flutter build ipa` and submit through Xcode or Transporter.

Store submissions need the bundle IDs finalised first — see
[CUSTOMIZE.md](CUSTOMIZE.md) §2. They cannot be changed after publishing.

The kit also has an in-app update path for APKs distributed outside the Play
Store (`make apk`, `make publish`, `make release`), driven by the `app_version`
module and `PUBLISH_SECRET`.

---

## Backups

The database is the only thing you cannot rebuild. Back it up before you have
customers, not after.

```bash
# Dump
docker compose exec -T postgres pg_dump -U marketkit marketkit | gzip > backup-$(date +%F).sql.gz

# Restore
gunzip -c backup-2026-07-28.sql.gz | docker compose exec -T postgres psql -U marketkit marketkit
```

A daily cron and off-server copies:

```cron
0 3 * * * cd /home/you/marketkit && docker compose exec -T postgres pg_dump -U marketkit marketkit | gzip > /backups/db-$(date +\%F).sql.gz
```

Uploaded files live in `api/uploads/` (or your R2/S3 bucket if configured) — back
those up too. Restoring a database whose files are gone leaves broken products.

---

## Updating

```bash
git pull
make deploy
```

Schema changes apply automatically on boot via GORM AutoMigrate. **Take a
database backup first** — AutoMigrate adds columns and indexes but will not undo
anything, and it cannot be rolled back.

---

## Before you go live

- [ ] Root `.env` and `api/.env` both have real values, matching passwords
- [ ] `JWT_SECRET` is a fresh 32-byte random value, not a placeholder
- [ ] `ADMIN_PASSWORD` changed after first login
- [ ] `APP_ENV=production` — this also disables the Swagger UI at `/docs`
- [ ] TLS working; HTTP redirects to HTTPS
- [ ] `CORS_ORIGIN` is your domain, not `*` or localhost
- [ ] Webhook registered and **tested**, signing secret set
- [ ] A real payment completed end-to-end in test mode, per gateway
- [ ] Database backup cron running, and a restore actually rehearsed
- [ ] `git status` clean — no `.env` or key files committed
- [ ] Alerting on `manual reconciliation needed` in the logs
      (see [WALLET.md](WALLET.md) — it means real money is out of sync)

---

## Troubleshooting

**API restarts in a loop** — `docker compose logs api`. Usually `DATABASE_URL`
pointing at `localhost` instead of `postgres:5432`, or a password mismatch
between the two `.env` files.

**nginx crash-loops** — the certificate paths in `nginx/nginx.conf` don't exist.
Check `/etc/letsencrypt/live/yourdomain.com/`.

**Payments succeed but nothing updates** — the webhook isn't arriving. Check the
gateway's delivery log, confirm the URL is publicly reachable, and confirm the
signing secret matches.

**Emails not sending** — the API logs SMTP failures at ERROR. Many hosts block
outbound port 25; use 587 with a real provider.

**Out of disk** — usually old Docker images. `docker system prune -a` (careful:
this removes unused images, not volumes).
