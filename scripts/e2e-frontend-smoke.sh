#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: missing required command: $1" >&2
    exit 1
  fi
}

require_cmd npx
require_cmd go
require_cmd curl
require_cmd python3

export CODEX_HOME="${CODEX_HOME:-$HOME/.codex}"
PWCLI="${PWCLI:-$CODEX_HOME/skills/playwright/scripts/playwright_cli.sh}"

if [[ ! -x "$PWCLI" ]]; then
  echo "Error: playwright wrapper not found or not executable: $PWCLI" >&2
  exit 1
fi

mapfile -t RELEASE_INFO < <(python3 - "$ROOT_DIR/CHANGELOG.md" <<'PY'
import pathlib
import re
import sys

text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
matches = re.findall(r"^## \[(v?[0-9][^\]]*)\] - ([0-9]{4}-[0-9]{2}-[0-9]{2})$", text, re.M)
if not matches:
    raise SystemExit("failed to parse latest release from CHANGELOG.md")
version, date = matches[0]
version = version.strip()
if not version.startswith("v"):
    print(f"v{version}")
    print(f"[{version}] - {date}")
    print(f"{version} - {date}")
else:
    print(version)
    print(f"[{version}] - {date}")
    print(f"{version.removeprefix('v')} - {date}")
PY
)

EXPECTED_VERSION="${RELEASE_INFO[0]}"
EXPECTED_CHANGELOG_RAW_MARKER="${RELEASE_INFO[1]}"
EXPECTED_CHANGELOG_HEADING="${RELEASE_INFO[2]}"
ARTIFACT_DIR="${ROOT_DIR}/output/playwright/frontend-smoke"
SERVER_LOG="${ARTIFACT_DIR}/server.log"
PLAYWRIGHT_LOG="${ARTIFACT_DIR}/playwright.log"
CCCLI_BIN="$(mktemp /tmp/qcc_plus-frontend-smoke-bin.XXXXXX)"
TEMP_HOME="$(mktemp -d /tmp/qcc_plus-frontend-smoke-home.XXXXXX)"
LISTEN_ADDR="${LISTEN_ADDR:-127.0.0.1:$(python3 - <<'PY'
import socket

sock = socket.socket()
sock.bind(("127.0.0.1", 0))
print(sock.getsockname()[1])
sock.close()
PY
)}"
BASE_URL="http://${LISTEN_ADDR}"
SERVER_PID=""

mkdir -p "$ARTIFACT_DIR"
: >"$SERVER_LOG"
: >"$PLAYWRIGHT_LOG"

cleanup() {
  local status=$?
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  (
    cd "$ARTIFACT_DIR"
    "$PWCLI" close >/dev/null 2>&1 || true
  )
  rm -rf "$TEMP_HOME"
  rm -f "$CCCLI_BIN"
  exit "$status"
}

trap cleanup EXIT

log_step() {
  echo "$1"
}

wait_for_server() {
  local attempt
  for attempt in $(seq 1 60); do
    if curl -fsS "${BASE_URL}/version" >/dev/null 2>&1; then
      return 0
    fi
    if [[ -n "$SERVER_PID" ]] && ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
      echo "Server exited unexpectedly. Recent log:" >&2
      tail -n 120 "$SERVER_LOG" >&2 || true
      return 1
    fi
    sleep 1
  done
  echo "Timed out waiting for ${BASE_URL} to become ready. Recent log:" >&2
  tail -n 120 "$SERVER_LOG" >&2 || true
  return 1
}

assert_no_cache_headers() {
  local endpoint="$1"
  local headers
  headers="$(curl -fsS -D - -o /dev/null "${BASE_URL}${endpoint}")"
  local normalized
  normalized="$(printf '%s' "$headers" | tr '[:upper:]' '[:lower:]')"
  if ! grep -q '^cache-control: .*no-store' <<<"$normalized"; then
    echo "Error: ${endpoint} missing no-store cache-control header" >&2
    printf '%s\n' "$headers" >&2
    exit 1
  fi
  if ! grep -q '^cache-control: .*no-cache' <<<"$normalized"; then
    echo "Error: ${endpoint} missing no-cache cache-control header" >&2
    printf '%s\n' "$headers" >&2
    exit 1
  fi
  if ! grep -q '^pragma: no-cache' <<<"$normalized"; then
    echo "Error: ${endpoint} missing pragma: no-cache header" >&2
    printf '%s\n' "$headers" >&2
    exit 1
  fi
  if ! grep -q '^expires: 0' <<<"$normalized"; then
    echo "Error: ${endpoint} missing expires: 0 header" >&2
    printf '%s\n' "$headers" >&2
    exit 1
  fi
}

log_step "[1/5] Building frontend dist ..."
bash "$ROOT_DIR/scripts/build-frontend.sh"

log_step "[2/5] Building local cccli binary ..."
(
  cd "$ROOT_DIR"
  go build -o "$CCCLI_BIN" ./cmd/cccli
)

log_step "[3/5] Starting isolated local server on ${BASE_URL} ..."
env \
  HOME="$TEMP_HOME" \
  LISTEN_ADDR="$LISTEN_ADDR" \
  UPSTREAM_BASE_URL="${UPSTREAM_BASE_URL:-https://api.anthropic.com}" \
  UPSTREAM_API_KEY="${UPSTREAM_API_KEY:-dummy}" \
  PROXY_HEALTH_INTERVAL_SEC="${PROXY_HEALTH_INTERVAL_SEC:-300}" \
  "$CCCLI_BIN" proxy >"$SERVER_LOG" 2>&1 &
SERVER_PID=$!

wait_for_server

log_step "[4/5] Verifying version and changelog endpoints ..."
VERSION_BODY="$(curl -fsS "${BASE_URL}/version")"
python3 - "$VERSION_BODY" "$EXPECTED_VERSION" <<'PY'
import json
import sys

payload = json.loads(sys.argv[1])
expected = sys.argv[2]
actual = payload.get("version")
if actual != expected:
    raise SystemExit(f"/version mismatch: expected {expected}, got {actual!r}")
print(f"  /version ok: {actual}")
PY

CHANGELOG_BODY="$(curl -fsS "${BASE_URL}/changelog")"
if [[ "$CHANGELOG_BODY" != *"$EXPECTED_CHANGELOG_RAW_MARKER"* ]]; then
  echo "Error: /changelog did not contain expected release marker: ${EXPECTED_CHANGELOG_RAW_MARKER}" >&2
  exit 1
fi
echo "  /changelog ok: found ${EXPECTED_CHANGELOG_RAW_MARKER}"

assert_no_cache_headers "/version"
echo "  /version cache headers ok"
assert_no_cache_headers "/changelog"
echo "  /changelog cache headers ok"

log_step "[5/5] Running Playwright frontend smoke ..."
json_quote() {
  python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$1"
}

BASE_URL_JSON="$(json_quote "$BASE_URL")"
EXPECTED_VERSION_JSON="$(json_quote "$EXPECTED_VERSION")"
EXPECTED_CHANGELOG_HEADING_JSON="$(json_quote "$EXPECTED_CHANGELOG_HEADING")"

PLAYWRIGHT_JS="$(cat <<'EOF'
async (page) => {
const baseUrl = __BASE_URL__;
const expectedVersion = __EXPECTED_VERSION__;
const expectedChangelogHeading = __EXPECTED_CHANGELOG_HEADING__;

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitForText(locator, expected, label) {
  for (let i = 0; i < 100; i += 1) {
    const text = ((await locator.textContent()) || '').trim();
    if (text === expected) {
      return text;
    }
    await sleep(100);
  }
  const actual = ((await locator.textContent()) || '').trim();
  throw new Error(`${label} mismatch: expected "${expected}", got "${actual || '<empty>'}"`);
}

async function waitForContains(locator, expected, label) {
  for (let i = 0; i < 100; i += 1) {
    const text = (await locator.textContent()) || '';
    if (text.includes(expected)) {
      return text;
    }
    await sleep(100);
  }
  const actual = (await locator.textContent()) || '';
  throw new Error(`${label} did not include "${expected}". Actual content starts with: ${actual.slice(0, 160)}`);
}

await page.waitForLoadState('domcontentloaded');

const loginVersion = await waitForText(page.locator('.login-version'), expectedVersion, 'login version');

await page.locator('input[name="username"]').fill('admin');
await page.locator('input[name="password"]').fill('admin123');
await Promise.all([
  page.waitForURL(/\/admin\/dashboard$/),
  page.getByRole('button', { name: '继续' }).click(),
]);

const sidebarVersion = await waitForText(page.locator('.sidebar-version'), expectedVersion, 'sidebar version');

await Promise.all([
  page.waitForURL(/\/changelog$/),
  page.getByRole('link', { name: '更新日志' }).click(),
]);
await waitForContains(page.locator('.changelog-markdown'), expectedChangelogHeading, 'changelog page');

await Promise.all([
  page.waitForURL(/\/admin\/nodes$/),
  page.getByRole('link', { name: '节点管理' }).click(),
]);
await page.getByRole('button', { name: '新增节点' }).click();

const nodeDialog = page.getByRole('dialog', { name: '新增节点' });
await nodeDialog.waitFor();

const nodeNameInput = nodeDialog.locator('input[placeholder="如：联通-北京"]');
const baseUrlInput = nodeDialog.locator('input[placeholder="https://api.anthropic.com"]');
await nodeNameInput.waitFor();
await baseUrlInput.waitFor();

const typedBaseUrl = 'https://example.com/focus-smoke';
await baseUrlInput.click();
await page.keyboard.type(typedBaseUrl, { delay: 30 });
await page.waitForTimeout(200);

const baseUrlStillFocused = await baseUrlInput.evaluate((el) => document.activeElement === el);
if (!baseUrlStillFocused) {
  throw new Error('base_url input lost focus while typing');
}

const baseUrlValue = await baseUrlInput.inputValue();
if (baseUrlValue !== typedBaseUrl) {
  throw new Error(`base_url value mismatch: expected "${typedBaseUrl}", got "${baseUrlValue}"`);
}

const nodeNameValue = await nodeNameInput.inputValue();
if (nodeNameValue !== '') {
  throw new Error(`node name input was modified unexpectedly: "${nodeNameValue}"`);
}

await nodeDialog.getByRole('button', { name: '添加映射' }).click();
const mappingFromInput = nodeDialog.locator('input[placeholder="源模型（选择或输入）"]').first();
const mappingToInput = nodeDialog.locator('input[placeholder="目标模型（选择或输入）"]').first();
await mappingFromInput.waitFor();

const typedMappingFrom = 'gpt-5';
await mappingFromInput.click();
await page.keyboard.type(typedMappingFrom, { delay: 30 });
await page.waitForTimeout(200);

const mappingStillFocused = await mappingFromInput.evaluate((el) => document.activeElement === el);
if (!mappingStillFocused) {
  throw new Error('model mapping source input lost focus while typing');
}

const mappingFromValue = await mappingFromInput.inputValue();
if (mappingFromValue !== typedMappingFrom) {
  throw new Error(`mapping source value mismatch: expected "${typedMappingFrom}", got "${mappingFromValue}"`);
}

const mappingToValue = await mappingToInput.inputValue();
if (mappingToValue !== '') {
  throw new Error(`mapping target input was modified unexpectedly: "${mappingToValue}"`);
}

return {
  loginVersion,
  sidebarVersion,
  expectedChangelogHeading,
  typedBaseUrl: baseUrlValue,
  typedMappingFrom: mappingFromValue,
  checkedUrl: `${baseUrl}/admin/nodes`,
};
}
EOF
)"

PLAYWRIGHT_JS="${PLAYWRIGHT_JS/__BASE_URL__/$BASE_URL_JSON}"
PLAYWRIGHT_JS="${PLAYWRIGHT_JS/__EXPECTED_VERSION__/$EXPECTED_VERSION_JSON}"
PLAYWRIGHT_JS="${PLAYWRIGHT_JS/__EXPECTED_CHANGELOG_HEADING__/$EXPECTED_CHANGELOG_HEADING_JSON}"

(
  cd "$ARTIFACT_DIR"
  "$PWCLI" close >/dev/null 2>&1 || true
  "$PWCLI" open "${BASE_URL}/login"
  "$PWCLI" resize 1440 1100
  "$PWCLI" snapshot >/dev/null
  PLAYWRIGHT_OUTPUT="$("$PWCLI" run-code "$PLAYWRIGHT_JS" 2>&1)"
  printf '%s\n' "$PLAYWRIGHT_OUTPUT" | tee -a "$PLAYWRIGHT_LOG"
  if grep -q '^### Error$' <<<"$PLAYWRIGHT_OUTPUT"; then
    echo "Error: Playwright smoke assertions failed" >&2
    exit 1
  fi
  if ! grep -q '"checkedUrl"' <<<"$PLAYWRIGHT_OUTPUT"; then
    echo "Error: Playwright smoke did not emit success payload" >&2
    exit 1
  fi
  "$PWCLI" close >/dev/null
)

echo "Done. Frontend smoke E2E checks completed."
