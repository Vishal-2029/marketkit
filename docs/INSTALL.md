# Install

Get MarketKit running locally in about ten minutes. Most of that is Docker
building images the first time — the commands themselves take seconds.

---

## What you need

| | Version | Needed for |
|---|---|---|
| Docker + Docker Compose | v2 (`docker compose`, not `docker-compose`) | Everything below |
| Node.js | 18+ | Admin panel |
| Flutter | 3.11+ | Mobile app |
| Go | 1.25+ | Only if you run the API outside Docker |
| openssl | any | Generating secrets during bootstrap |

Check with:

```bash
docker compose version && node -v && flutter --version && go version
```

You do **not** need a payment gateway account to get started. The API boots
fine without one and only errors when a payment is actually attempted.

---

## One command

```bash
make quickstart
```

That does five things:

1. Generates `.env` and `api/.env` with fresh random secrets
2. Builds and starts the API, PostgreSQL, Redis and Mailhog
3. Waits for the API to report healthy
4. Creates the database schema
5. Seeds demo data — sellers, products, purchases, wallets, platform revenue

First run takes a few minutes for the Docker build. Later runs take seconds.

### When it finishes

| | URL |
|---|---|
| API | http://localhost:3000 |
| Health check | http://localhost:3000/health |
| API docs (Swagger UI, dev only) | http://localhost:3000/docs/index.html |
| Mailhog — catches every outgoing email | http://localhost:8025 |

The admin panel runs separately:

```bash
make web-dev     # http://localhost:5173
```

### Demo accounts

Password `demo1234` for all of them:

```
seller1@demo.marketkit.test  …  seller5@demo.marketkit.test
buyer1@demo.marketkit.test   …  buyer8@demo.marketkit.test
```

The admin account is `admin@example.com`. Its password is generated per install
and printed during bootstrap — you can also read it from `api/.env`:

```bash
grep ADMIN_PASSWORD api/.env
```

### Logging in

Login is two-step: email + password, then a one-time code emailed to you.
Locally that email goes to Mailhog, not to a real inbox.

```bash
curl -X POST http://localhost:3000/api/v1/user/auth/send-otp \
  -H 'Content-Type: application/json' \
  -d '{"email":"buyer1@demo.marketkit.test","password":"demo1234"}'
```

Then open http://localhost:8025 and read the code out of the message.

---

## Everyday commands

| Command | What it does |
|---|---|
| `make up` | Start the stack (no rebuild) |
| `make down` | Stop the stack, keep the data |
| `make down-v` | Stop **and wipe the database** |
| `make logs` | Tail all logs |
| `make logs-api` | Tail the API only |
| `make db-shell` | psql inside the Postgres container |
| `make seed-demo` | Add demo data to an existing database |
| `make seed-demo-reset` | Remove demo data, then re-seed |
| `make test` | Go tests against a throwaway Postgres |
| `make swagger` | Regenerate the API docs |

`make help` lists everything.

---

## Running the mobile app

The app needs to know where the API is. `localhost` means the phone itself, so
an emulator or device cannot use it:

```bash
cd app
flutter pub get

# Android emulator — 10.0.2.2 is the host machine
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:3000/api/v1

# iOS simulator — shares the host network
flutter run --dart-define=API_BASE_URL=http://localhost:3000/api/v1

# Physical device — your machine's LAN IP, same Wi-Fi
flutter run --dart-define=API_BASE_URL=http://192.168.1.50:3000/api/v1
```

Find your LAN IP with `ip -4 addr` (Linux), `ipconfig getifaddr en0` (macOS) or
`ipconfig` (Windows).

---

## Running the API without Docker

You still need PostgreSQL somewhere. The compose stack exposes it on `5433`:

```bash
make up                 # postgres + redis + mailhog
cd api
go run ./cmd/api
```

Point `DATABASE_URL` in `api/.env` at `localhost:5433` instead of
`postgres:5432` when the API runs outside the compose network.

---

## Troubleshooting

### `port is already allocated`

Something else holds the port. Either stop it, or change the port:

```bash
# find the culprit
docker ps --format '{{.Names}}\t{{.Ports}}'
ss -ltnp | grep :3000

# or move MarketKit
sed -i 's/^PORT=3000/PORT=3001/' .env
make down && make up
```

### `POSTGRES_PASSWORD is required`

The root `.env` is missing. Compose fails closed here on purpose rather than
falling back to a default password.

```bash
make bootstrap
```

### `DATABASE_URL is required` / API exits immediately

`api/.env` is missing or incomplete. `make bootstrap` regenerates it without
touching a file that already exists.

### API container restarts in a loop

```bash
make logs-api
```

Most common causes: Postgres not ready yet (wait ~30s on first boot), or
`DATABASE_URL` pointing at `localhost` from inside the container — it must be
`postgres:5432` there.

### Emails don't appear in Mailhog

Check `SMTP_HOST=mailhog` and `SMTP_PORT=1025` in `api/.env`, and that the
container is up (`docker compose ps`). Mailhog holds mail in memory only, so
restarting it empties the inbox.

### Admin panel shows network or CORS errors

The API must be running, and its `CORS_ORIGIN` must include the panel's origin.
The dev compose sets `http://localhost:5173,http://localhost:8081` already. If
you changed `PORT`, set `VITE_API_URL` for the panel to match:

```bash
echo 'VITE_API_URL=http://localhost:3001/api/v1' > web/.env
```

### `make test` fails to connect

The test suite runs against its own throwaway Postgres on port `5434`, separate
from your dev data. If a previous run left it behind:

```bash
make test-db-down && make test
```

### Start completely fresh

```bash
make down-v          # stops everything and deletes the database volume
make quickstart
```

---

## Next

- [WALLET.md](WALLET.md) — how the money layer works before you change it
- [CUSTOMIZE.md](CUSTOMIZE.md) — name, logo, colours, categories, currency
- [DEPLOY.md](DEPLOY.md) — putting it on a server
