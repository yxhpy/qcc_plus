#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_PATH="$ROOT_DIR/scripts/deploy-server.sh"
TMP_DIR="$(mktemp -d)"
LAST_STATUS=0
LAST_OUTPUT=""

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

fail() {
  printf '[FAIL] %s\n' "$*" >&2
  if [[ -n "$LAST_OUTPUT" && -f "$LAST_OUTPUT" ]]; then
    printf '--- last output ---\n' >&2
    cat "$LAST_OUTPUT" >&2
    printf '\n-------------------\n' >&2
  fi
  exit 1
}

pass() {
  printf '[PASS] %s\n' "$*"
}

run_case() {
  local name="$1"
  shift

  LAST_OUTPUT="$TMP_DIR/${name}.log"
  set +e
  bash "$SCRIPT_PATH" "$@" >"$LAST_OUTPUT" 2>&1
  LAST_STATUS=$?
  set -e
}

assert_exit_code() {
  local expected="$1"
  if [[ "$LAST_STATUS" -ne "$expected" ]]; then
    fail "expected exit code $expected, got $LAST_STATUS"
  fi
}

assert_nonzero_exit() {
  if [[ "$LAST_STATUS" -eq 0 ]]; then
    fail "expected non-zero exit code"
  fi
}

assert_contains() {
  local pattern="$1"
  if ! grep -qE "$pattern" "$LAST_OUTPUT"; then
    fail "expected output to match pattern: $pattern"
  fi
}

assert_deploy_id_format() {
  local deploy_id
  deploy_id="$(grep -Eo 'deploy_id=[0-9]{14}-[0-9a-f]{6}' "$LAST_OUTPUT" | head -n 1 | cut -d= -f2)"
  if [[ -z "$deploy_id" ]]; then
    fail "deploy_id line not found"
  fi
  if [[ ! "$deploy_id" =~ ^[0-9]{14}-[0-9a-f]{6}$ ]]; then
    fail "invalid deploy_id format: $deploy_id"
  fi
}

if [[ ! -f "$SCRIPT_PATH" ]]; then
  fail "script not found: $SCRIPT_PATH"
fi

run_case help --help
assert_exit_code 0
assert_contains '^Usage: ./scripts/deploy-server\.sh \[--dry-run\] <test\|prod>$'
pass '--help outputs usage'

run_case missing_args
assert_nonzero_exit
assert_contains 'missing environment argument'
assert_contains '^Usage: ./scripts/deploy-server\.sh \[--dry-run\] <test\|prod>$'
pass 'missing args fail with usage'

run_case invalid_env invalid-env
assert_nonzero_exit
assert_contains 'invalid environment or option: invalid-env'
assert_contains '^Usage: ./scripts/deploy-server\.sh \[--dry-run\] <test\|prod>$'
pass 'invalid env fails with usage'

run_case dry_run --dry-run test
assert_exit_code 0
assert_contains 'environment=test branch=test port=8001 compose_file=docker-compose\.test\.yml project=qcc_test'
assert_contains 'dry run enabled; skipping git sync, build, and deploy steps'
assert_deploy_id_format
pass 'dry run exposes deploy_id and deployment context'

printf '[PASS] all deploy script checks passed\n'
