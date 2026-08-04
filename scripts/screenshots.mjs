#!/usr/bin/env node
/**
 * Capture admin-panel screenshots for the landing page.
 *
 *   make up && make seed-demo && make web-dev      # stack + panel running
 *   node scripts/screenshots.mjs
 *
 * Writes PNGs into site/screenshots/. Re-run after any UI change so the sales
 * page never shows a stale product.
 *
 * Auth: signs in over the API rather than by filling the login form. The panel
 * keeps its access token in memory only and restores the session on mount from
 * the httpOnly refresh cookie, so obtaining that cookie is enough — Playwright's
 * request context shares the browser's cookie jar.
 *
 * Env: PANEL, API, MAILHOG, ADMIN_EMAIL, ADMIN_PASSWORD (defaults below).
 */
import { chromium } from "../web/node_modules/playwright-core/index.mjs";
import { mkdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const OUT = join(ROOT, "site", "screenshots");

const PANEL = process.env.PANEL ?? "http://localhost:5173";
const API = process.env.API ?? "http://localhost:3001";
const MAILHOG = process.env.MAILHOG ?? "http://localhost:8025";
const EMAIL = process.env.ADMIN_EMAIL ?? "admin@example.com";
const PASSWORD = process.env.ADMIN_PASSWORD;

const SHOTS = [
  { path: "/login", file: "admin-login.png", label: "sign in", anon: true },
  { path: "/", file: "admin-dashboard.png", label: "dashboard" },
  { path: "/products", file: "admin-products.png", label: "products & purchases" },
  { path: "/withdrawals", file: "admin-withdrawals.png", label: "withdrawals" },
];

if (!PASSWORD) {
  console.error("ADMIN_PASSWORD is required.  ADMIN_PASSWORD=$(grep ADMIN_PASSWORD api/.env | cut -d= -f2) node scripts/screenshots.mjs");
  process.exit(1);
}

/** Pull the newest OTP for an address out of Mailhog. */
async function latestOtp(email) {
  const res = await fetch(`${MAILHOG}/api/v2/search?kind=to&query=${encodeURIComponent(email)}&limit=50`);
  const { items = [] } = await res.json();
  if (!items.length) throw new Error(`no mail for ${email} — is Mailhog running?`);

  items.sort((a, b) => String(b.Created).localeCompare(String(a.Created)));
  // Bodies are quoted-printable; soft line breaks split the code across lines.
  const body = items[0].Content.Body.replace(/=\r?\n/g, "").replace(/=([0-9A-F]{2})/g,
    (_, h) => String.fromCharCode(parseInt(h, 16)));
  // Anchor on the hidden preheader — a bare \d{6} also matches hex colours.
  const m = body.match(/Your OTP is\s*(\d{6})/);
  if (!m) throw new Error("could not find an OTP in the newest email");
  return m[1];
}

const browser = await chromium.launch({ channel: "chrome" });
const ctx = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 2, // retina-crisp on the sales page
});
const page = await ctx.newPage();

try {
  mkdirSync(OUT, { recursive: true });

  // Anonymous pages first — once the refresh cookie exists the panel bounces
  // straight past /login.
  for (const shot of SHOTS.filter((x) => x.anon)) {
    await page.goto(PANEL + shot.path, { waitUntil: "networkidle" });
    await page.waitForTimeout(1500);
    await page.screenshot({ path: join(OUT, shot.file) });
    console.log(`  ✓ ${shot.file}  (${shot.label})`);
  }

  console.log("signing in over the API…");
  const send = await ctx.request.post(`${API}/api/v1/auth/send-otp`, {
    data: { email: EMAIL, password: PASSWORD },
  });
  if (!send.ok()) throw new Error(`send-otp failed: ${send.status()} ${await send.text()}`);

  await new Promise((r) => setTimeout(r, 1500)); // let the mail land
  const otp = await latestOtp(EMAIL);

  const verify = await ctx.request.post(`${API}/api/v1/auth/verify-otp`, {
    data: { email: EMAIL, otp },
  });
  if (!verify.ok()) throw new Error(`verify-otp failed: ${verify.status()} ${await verify.text()}`);

  const cookies = await ctx.cookies();
  if (!cookies.some((c) => c.name.includes("refresh"))) {
    throw new Error("no refresh cookie was set — the panel cannot restore a session");
  }
  console.log("  signed in, refresh cookie stored");

  mkdirSync(OUT, { recursive: true });

  for (const shot of SHOTS.filter((x) => !x.anon)) {
    await page.goto(PANEL + shot.path, { waitUntil: "networkidle" });
    // The panel restores its session on mount, then fetches; wait for the
    // login screen to be gone rather than a fixed sleep.
    await page.waitForTimeout(2500);

    if (!shot.anon && page.url().includes("/login")) {
      throw new Error(`still on /login when capturing ${shot.path} — session not restored`);
    }
    await page.screenshot({ path: join(OUT, shot.file) });
    console.log(`  ✓ ${shot.file}  (${shot.label})`);
  }

  console.log(`\nwrote ${SHOTS.length} screenshots to site/screenshots/`);
} finally {
  await browser.close();
}
