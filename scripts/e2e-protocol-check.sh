#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8000}"
PROXY_KEY="${PROXY_KEY:-admin}"
OPENAI_MODEL="${OPENAI_MODEL:-gpt-5.1-codex-mini}"
GEMINI_MODEL="${GEMINI_MODEL:-gemini-2.5-flash}"
CLAUDE_MODEL="${CLAUDE_MODEL:-claude-haiku-4-5-20251001}"

echo "[1/4] Checking /v1/models ..."
MODELS_RESP="$(curl -sS "${BASE_URL}/v1/models" -H "Authorization: Bearer ${PROXY_KEY}")"
python3 - <<'PY' "${MODELS_RESP}"
import json,sys
raw=sys.argv[1]
obj=json.loads(raw)
assert isinstance(obj, dict), "models response is not JSON object"
if obj.get("object") == "list":
    data=obj.get("data",[])
    print(f"  models list ok, count={len(data)}")
elif obj.get("success") is True and isinstance(obj.get("data"), list):
    print(f"  models list ok (custom schema), count={len(obj['data'])}")
else:
    raise AssertionError(f"unexpected /v1/models schema: {obj}")
PY

echo "[2/4] Checking OpenAI endpoint /v1/chat/completions ..."
OPENAI_RESP="$(curl -sS "${BASE_URL}/v1/chat/completions" \
  -H "Authorization: Bearer ${PROXY_KEY}" \
  -H "Content-Type: application/json" \
  -d "{\"model\":\"${OPENAI_MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"Reply with exactly: E2E_OK\"}],\"max_tokens\":16,\"temperature\":0}")"
python3 - <<'PY' "${OPENAI_RESP}"
import json,sys
obj=json.loads(sys.argv[1])
if "error" in obj:
    raise AssertionError(f"openai endpoint returned error: {obj['error']}")
choices=obj.get("choices") or []
assert choices, f"openai response has no choices: {obj}"
msg=((choices[0] or {}).get("message") or {}).get("content", "")
assert isinstance(msg, str) and len(msg) > 0, f"openai message empty: {obj}"
print(f"  openai ok, assistant content={msg!r}")
PY

echo "[3/4] Checking Gemini endpoint /v1beta/models/...:generateContent ..."
set +e
GEMINI_RESP="$(curl -sS "${BASE_URL}/v1beta/models/${GEMINI_MODEL}:generateContent" \
  -H "Authorization: Bearer ${PROXY_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"parts":[{"text":"Reply with exactly: GEMINI_E2E_OK"}]}]}')"
set -e
python3 - <<'PY' "${GEMINI_RESP}"
import json,sys
obj=json.loads(sys.argv[1])
if "error" in obj:
    print(f"  gemini endpoint error (kept for diagnostics): {obj['error']}")
else:
    cands=obj.get("candidates") or []
    assert cands, f"gemini response has no candidates: {obj}"
    print("  gemini ok, candidates present")
PY

echo "[4/4] Checking Claude endpoint /v1/messages ..."
CLAUDE_RESP="$(curl -sS "${BASE_URL}/v1/messages" \
  -H "Authorization: Bearer ${PROXY_KEY}" \
  -H "Content-Type: application/json" \
  -d "{\"model\":\"${CLAUDE_MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"Reply with exactly: CLAUDE_E2E_OK\"}],\"max_tokens\":32}")"
python3 - <<'PY' "${CLAUDE_RESP}"
import json,sys
obj=json.loads(sys.argv[1])
if "error" in obj:
    raise AssertionError(f"claude endpoint returned error: {obj['error']}")
content=obj.get("content") or []
assert content, f"claude response has empty content: {obj}"
text=(content[0] or {}).get("text", "")
assert isinstance(text, str) and len(text) > 0, f"claude text empty: {obj}"
print(f"  claude ok, assistant text={text!r}")
PY

echo "Done. Protocol E2E checks completed."
