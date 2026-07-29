# MarketKit documentation

Start here.

## Getting started

| | |
|---|---|
| **[INSTALL.md](INSTALL.md)** | Running locally in ten minutes, plus troubleshooting |
| **[CUSTOMIZE.md](CUSTOMIZE.md)** | Name, logo, colours, categories, currency, gateway — and removing modules you don't need |
| **[DEPLOY.md](DEPLOY.md)** | VPS, TLS, webhooks, app builds, backups, pre-launch checklist |

## Understanding the code

| | |
|---|---|
| **[WALLET.md](WALLET.md)** | The money layer: two ledgers, their invariants, the fee split, refunds. **Read before changing anything financial.** |
| **[ARCHITECTURE.md](ARCHITECTURE.md)** | Full system tour — modules, auth, API surface, database tables |
| **[TESTING.md](TESTING.md)** | What is tested, why each check exists, and what is deliberately not covered |

## Optional features

| | |
|---|---|
| **[FIREBASE_SETUP.md](FIREBASE_SETUP.md)** | Setting up Firebase, if you want push notifications |
| **[PUSH_NOTIFICATIONS.md](PUSH_NOTIFICATIONS.md)** | How notifications work in the code, plus troubleshooting |

Push notifications are entirely optional. The API starts and runs fine without
Firebase configured — it logs that notifications are disabled and carries on.

## API reference

The API documents itself. With the stack running in development:

```
http://localhost:3000/docs/index.html
```

It is disabled when `APP_ENV=production`, because it enumerates every route and
internal field name. Regenerate after changing annotations with `make swagger`.
