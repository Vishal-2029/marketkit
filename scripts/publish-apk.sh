#!/usr/bin/env bash
# Publish the built APK to the server so users get the in-app update prompt.
# Usage (from repo root): ./scripts/publish-apk.sh
#
# Reads PUBLISH_SECRET from api/.env automatically — no manual token needed.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="${ROOT_DIR}/app"
ENV_FILE="${ROOT_DIR}/api/.env"
API_BASE_URL="${API_BASE_URL:-https://api.example.com/api/v1}"
APK="${APP_DIR}/build/app/outputs/apk/release/app-release.apk"

# ── Read PUBLISH_SECRET from api/.env ─────────────────────────────────────────
if [[ -z "${PUBLISH_SECRET:-}" ]]; then
  if [[ -f "${ENV_FILE}" ]]; then
    PUBLISH_SECRET="$(grep '^PUBLISH_SECRET=' "${ENV_FILE}" | cut -d'=' -f2- | tr -d '[:space:]' || true)"
  fi
fi

if [[ -z "${PUBLISH_SECRET:-}" ]]; then
  echo "ERROR: PUBLISH_SECRET not set in api/.env" >&2
  exit 1
fi

# ── APK check ─────────────────────────────────────────────────────────────────
if [[ ! -f "${APK}" ]]; then
  echo "ERROR: APK not found at ${APK}" >&2
  echo "Run 'make apk' first." >&2
  exit 1
fi

# ── Read version from pubspec.yaml ────────────────────────────────────────────
RAW_VERSION="$(grep '^version:' "${APP_DIR}/pubspec.yaml" | awk '{print $2}')"
VERSION_NAME="${RAW_VERSION%%+*}"
BUILD_NUMBER="${RAW_VERSION##*+}"

echo ""
echo "==> Publishing APK"
echo "    Version : ${VERSION_NAME}"
echo "    Build   : ${BUILD_NUMBER}"
echo "    APK     : ${APK}"
echo "    API     : ${API_BASE_URL}"
echo ""

# ── Upload ────────────────────────────────────────────────────────────────────
RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X POST "${API_BASE_URL}/admin/apk/publish" \
  -H "X-Publish-Secret: ${PUBLISH_SECRET}" \
  -F "apk_file=@${APK};type=application/vnd.android.package-archive" \
  -F "version_name=${VERSION_NAME}" \
  -F "build_number=${BUILD_NUMBER}" \
  -F "release_notes=Version ${VERSION_NAME} (build ${BUILD_NUMBER})" \
  -F "is_mandatory=false")

HTTP_CODE="${RESPONSE##*$'\n'}"
BODY="${RESPONSE%$'\n'*}"

if [[ "${HTTP_CODE}" == "201" ]]; then
  echo "==> Published! Users will see the update prompt on next app launch."
  echo ""
elif [[ "${HTTP_CODE}" == "401" ]]; then
  echo "ERROR: Invalid publish secret (HTTP 401)." >&2
  exit 1
else
  echo "ERROR: Upload failed (HTTP ${HTTP_CODE})" >&2
  echo "${BODY}" >&2
  exit 1
fi
