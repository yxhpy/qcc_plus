#!/usr/bin/env bash
# v1.0 - Scan staged additions for secret leaks

set -uo pipefail

DIFF_OUTPUT="$(git diff --staged --unified=0 --no-color --diff-filter=ACMR)"

if [[ -z "${DIFF_OUTPUT}" ]]; then
  exit 0
fi

# Check each added line for secrets
found=0
current_file=""
line_num=0

while IFS= read -r line; do
  # Track file name
  if [[ "$line" =~ ^\+\+\+\ b/ ]]; then
    current_file="${line#\+\+\+ b/}"
    continue
  fi
  # Track line number from hunk header
  if [[ "$line" =~ ^@@.*\+([0-9]+) ]]; then
    line_num="${BASH_REMATCH[1]}"
    continue
  fi
  # Only check added lines (skip +++ headers)
  if [[ "$line" =~ ^\+ ]] && ! [[ "$line" =~ ^\+\+\+ ]]; then
    added="${line:1}"
    # Check for secrets (case insensitive for password)
    secret_type=""
    if echo "$added" | grep -qE 'sk-[A-Za-z0-9_-]{8,}'; then
      secret_type="API Key (sk-...)"
    elif echo "$added" | grep -qE 'gh[pou]_[A-Za-z0-9_]{8,}'; then
      secret_type="GitHub Token"
    elif echo "$added" | grep -qE 'AKIA[0-9A-Z]{16}'; then
      secret_type="AWS Access Key"
    elif echo "$added" | grep -qi 'BEGIN.*PRIVATE KEY' && ! echo "$added" | grep -qE '(grep|awk|sed|pattern|regex)'; then
      secret_type="Private Key (PEM)"
    elif echo "$added" | grep -qiE 'password[[:space:]]*=[[:space:]]*[^[:space:]]+'; then
      # Skip if it's a comment
      if ! echo "$added" | grep -qE '^\s*#'; then
        secret_type="Password Assignment"
      fi
    fi

    if [[ -n "$secret_type" ]]; then
      # Redact the secret for display
      redacted=$(echo "$added" | sed -E 's/sk-[A-Za-z0-9_-]{8,}/sk-****/g; s/gh[pou]_[A-Za-z0-9_]{8,}/gh*_****/g; s/AKIA[0-9A-Z]{16}/AKIA****/g')
      echo "  ${current_file}:${line_num}: [${secret_type}] ${redacted}" >&2
      found=1
    fi
    line_num=$((line_num + 1))
  fi
done <<< "${DIFF_OUTPUT}"

if [[ "$found" -eq 1 ]]; then
  echo "" >&2
  echo "ERROR: Secret leak detected in staged changes! Commit blocked." >&2
  exit 1
fi

exit 0
