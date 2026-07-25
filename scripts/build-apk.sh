#!/usr/bin/env bash
# Build release APK for the Flutter app.
# Usage (from repo root): ./scripts/build-apk.sh
# Override API: API_BASE_URL=https://example.com/api/v1 ./scripts/build-apk.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="${ROOT_DIR}/app"
API_BASE_URL="${API_BASE_URL:-https://api.example.com/api/v1}"
GOOGLE_SERVICES="${APP_DIR}/android/app/google-services.json"

if [[ ! -f "${GOOGLE_SERVICES}" ]]; then
  echo "ERROR: missing ${GOOGLE_SERVICES}" >&2
  echo "Download from Firebase Console — see docs/PUSH_NOTIFICATIONS.md" >&2
  exit 1
fi

cd "${APP_DIR}"

echo "==> flutter clean"
flutter clean

echo "==> flutter build apk --release (API_BASE_URL=${API_BASE_URL})"
flutter build apk --release --dart-define="API_BASE_URL=${API_BASE_URL}"

APK="${APP_DIR}/build/app/outputs/apk/release/app-release.apk"
echo ""
echo "==> APK ready: ${APK}"
