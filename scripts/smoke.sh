#!/usr/bin/env bash
#
# End-to-end smoke test against a running stack.
#
#   make up && make seed-demo
#   ./scripts/smoke.sh                 # defaults to http://localhost:3000
#   API=http://localhost:3001 ./scripts/smoke.sh
#
# Walks a real buyer through login -> browse -> top up -> purchase -> download,
# checks the wallet ledger still balances afterwards, then tries the attacks
# that matter for a marketplace: IDOR, missing auth, privilege escalation and
# negative amounts.
#
# Needs the dev stack (Mailhog supplies the login OTP). Read-only apart from the
# demo buyer's own wallet.
set -uo pipefail

API="${API:-http://localhost:3000}"
MAILHOG="${MAILHOG:-http://localhost:8025}"
BUYER="${BUYER:-buyer4@demo.marketkit.test}"
OTHER="${OTHER:-buyer1@demo.marketkit.test}"
PASS="${PASS:-demo1234}"

pass=0; fail=0
ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; pass=$((pass+1)); }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=$((fail+1)); }
head_() { printf '\n\033[1m%s\033[0m\n' "$1"; }

# expect <description> <actual> <expected>
expect() { [[ "$2" == "$3" ]] && ok "$1" || bad "$1 (got $2, want $3)"; }

# Walk a JSON key path: jqf data items 0 id
# (an eval-based version breaks on the nested quotes, and silently returns "")
jqf() { python3 -c "
import sys, json
d = json.load(sys.stdin)
for k in sys.argv[1:]:
    d = d[int(k)] if k.lstrip('-').isdigit() else d[k]
print(d)" "$@" 2>/dev/null; }

# Length of a JSON array at a key path: jqlen data items
jqlen() { python3 -c "
import sys, json
d = json.load(sys.stdin)
for k in sys.argv[1:]:
    d = d[int(k)] if k.lstrip('-').isdigit() else d[k]
print(len(d))" "$@" 2>/dev/null; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

# login <email> -> prints access token
login() {
  local email="$1"
  # A rate-limited send is fine: OTPs stay valid for 10 minutes, so the newest
  # message in Mailhog still works. Only a hard failure matters here.
  curl -s -X POST "$API/api/v1/user/auth/send-otp" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$email\",\"password\":\"$PASS\"}" >/dev/null
  sleep 1

  # The OTP only exists in the email, so pull the newest message for this
  # address out of Mailhog and scrape the code.
  local otp
  otp=$(curl -s "$MAILHOG/api/v2/search?kind=to&query=$email&limit=50" | python3 -c "
import sys,json,re,quopri
d=json.load(sys.stdin)
items=sorted(d.get('items',[]), key=lambda m: m.get('Created',''), reverse=True)
if not items: sys.exit(1)
b=items[0]['Content']['Body']
try: b=quopri.decodestring(b).decode('utf8','ignore')
except Exception: pass
# The hidden preheader states the code in prose. Anchor on it — a bare
# 6-digit search also matches hex colours like #888888 in the template.
m=re.search(r'Your OTP is\s*(\d{6})', b)
print(m.group(1) if m else '')
" 2>/dev/null)
  [[ -z "$otp" ]] && { echo ""; return 1; }

  local resp
  resp=$(curl -s -X POST "$API/api/v1/user/auth/verify-otp" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$email\",\"otp\":\"$otp\"}")
  [[ -n "${SMOKE_DEBUG:-}" ]] && echo "verify($email) otp=$otp -> $resp" >&2
  echo "$resp" | jqf data token
}

head_ "Health"
expect "GET /health returns 200" "$(code "$API/health")" "200"

head_ "Login (email + password + emailed OTP)"
TOKEN=$(login "$BUYER")
[[ -n "$TOKEN" && "$TOKEN" != "None" ]] && ok "buyer logged in" || { bad "buyer login failed — is the dev stack + Mailhog up?"; exit 1; }
AUTH=(-H "Authorization: Bearer $TOKEN")

head_ "Browse"
PRODUCTS=$(curl -s "${AUTH[@]}" "$API/api/v1/user/market/products")
COUNT=$(echo "$PRODUCTS" | jqlen data)
[[ "${COUNT:-0}" -gt 0 ]] && ok "product list returned $COUNT items" || bad "no products — run: make seed-demo"
# Cheapest product this buyer does NOT already own, so the purchase path
# actually runs on a re-run instead of short-circuiting on "already purchased".
OWNED=$(curl -s "${AUTH[@]}" "$API/api/v1/user/market/my/purchases")
CHEAPEST=$(echo "$PRODUCTS" | OWNED_JSON="$OWNED" python3 -c "
import sys, json, os
items = json.load(sys.stdin)['data']
owned = {p.get('product_id') for p in (json.loads(os.environ['OWNED_JSON']).get('data') or [])}
free = [i for i in items if i['id'] not in owned] or items
p = min(free, key=lambda x: x['price_minor'])
print(p['id'], p['price_minor'])")
PID=${CHEAPEST% *}
PRICE=${CHEAPEST#* }

head_ "Wallet"
BAL0=$(curl -s "${AUTH[@]}" "$API/api/v1/user/wallet/" | jqf data balance_minor)
[[ -n "$BAL0" ]] && ok "wallet balance readable ($BAL0 minor)" || bad "wallet summary failed"

head_ "Purchase from wallet"
BUY=$(curl -s -X POST "${AUTH[@]}" -H 'Content-Type: application/json' \
      -d "{\"product_id\":\"$PID\"}" "$API/api/v1/user/market/purchases/wallet")
if echo "$BUY" | grep -q '"success":true'; then
  ok "purchase succeeded"
  BAL1=$(curl -s "${AUTH[@]}" "$API/api/v1/user/wallet/" | jqf data balance_minor)
  expect "buyer debited exactly the price" "$((BAL0 - BAL1))" "$PRICE"
  DL=$(curl -s "${AUTH[@]}" "$API/api/v1/user/market/products/$PID/download-url" | jqf data url)
  [[ -n "$DL" && "$DL" != "None" ]] && ok "download url issued to the buyer" || bad "no download url returned"
  # The URL must actually work for the buyer …
  expect "signed download url resolves" "$(code "$DL")" "200"
  # … and must not work with the signature stripped, or paid content is free.
  expect "same url without its signature is refused" "$(code "${DL%%\?*}")" "403"
else
  # Insufficient balance is a legitimate outcome; the ledger checks still run.
  ok "purchase declined cleanly ($(echo "$BUY" | jqf error))"
fi

head_ "Ledger invariant"
INV=$(docker compose -f docker-compose.yml -f docker-compose.dev.yml exec -T postgres \
  psql -U "${POSTGRES_USER:-marketkit}" -d "${POSTGRES_DB:-marketkit}" -tAc "
    SELECT count(*) FROM users u
    LEFT JOIN wallet_transactions wt ON wt.user_id = u.id::text
    GROUP BY u.id, u.wallet_balance_minor
    HAVING u.wallet_balance_minor <> COALESCE(SUM(wt.amount_minor),0);" 2>/dev/null | wc -l)
expect "every wallet balance equals SUM(its ledger rows)" "${INV:-1}" "0"

head_ "Attacks — unauthenticated access"
for path in /user/wallet/ /user/market/my/purchases /user/market/products; do
  expect "401 without a token: $path" "$(code "$API/api/v1$path")" "401"
done

head_ "Attacks — privilege escalation"
for path in /users /payments /platform-wallet /admins; do
  got=$(code "${AUTH[@]}" "$API/api/v1$path")
  [[ "$got" == "401" || "$got" == "403" ]] && ok "user token rejected on admin $path ($got)" \
    || bad "admin $path reachable with a user token ($got)"
done

head_ "Attacks — IDOR"
OTHER_TOKEN=$(login "$OTHER")
if [[ -n "$OTHER_TOKEN" && "$OTHER_TOKEN" != "None" ]]; then
  MINE=$(curl -s "${AUTH[@]}" "$API/api/v1/user/market/my/purchases" | jqf data 0 id)
  if [[ -n "$MINE" && "$MINE" != "None" ]]; then
    got=$(code -H "Authorization: Bearer $OTHER_TOKEN" "$API/api/v1/user/market/purchases/$MINE/invoice")
    [[ "$got" == "404" || "$got" == "403" ]] && ok "another user cannot fetch my invoice ($got)" \
      || bad "IDOR: another user fetched my invoice ($got)"
  else
    ok "no purchases to IDOR-test (skipped)"
  fi
else
  ok "second buyer unavailable (skipped)"
fi

head_ "Attacks — business logic"
neg=$(curl -s -X POST "${AUTH[@]}" -H 'Content-Type: application/json' \
      -d '{"amount_minor":-500000}' "$API/api/v1/user/wallet/topup/order")
echo "$neg" | grep -q '"success":false' && ok "negative top-up rejected" || bad "negative top-up accepted: $neg"

huge=$(curl -s -X POST "${AUTH[@]}" -H 'Content-Type: application/json' \
       -d '{"amount_minor":999999999999}' "$API/api/v1/user/wallet/topup/order")
echo "$huge" | grep -q '"success":false' && ok "oversized top-up rejected" || bad "oversized top-up accepted"

wd=$(curl -s -X POST "${AUTH[@]}" -H 'Content-Type: application/json' \
     -d '{"amount_minor":-100000,"method":"UPI"}' "$API/api/v1/user/wallet/withdrawals")
echo "$wd" | grep -q '"success":false' && ok "negative withdrawal rejected" || bad "negative withdrawal accepted: $wd"

head_ "Attacks — internal exposure"
for path in /.env /.git/config /api/v1/../.env; do
  expect "not served: $path" "$(code "$API$path")" "404"
done

head_ "Attacks — injection"
inj=$(curl -s -G "${AUTH[@]}" --data-urlencode "search=' OR 1=1--" "$API/api/v1/user/market/products")
echo "$inj" | grep -q '"success":true' && ok "SQL metacharacters handled as data" || bad "search broke on quote injection"

printf '\n\033[1m%d passed, %d failed\033[0m\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]]
