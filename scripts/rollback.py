#!/usr/bin/env python3
# v1.0 - Standardized rollback tool (preview + execute)

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path
from typing import Any

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
DEPLOYMENT_FILE = PROJECT_ROOT / ".atlas" / "deployments.jsonl"

ALLOWED_ENVS = {"test", "prod"}


def _run_git(args: list[str]) -> tuple[bool, str]:
    try:
        result = subprocess.run(
            ["git", *args],
            cwd=PROJECT_ROOT,
            check=False,
            capture_output=True,
            text=True,
        )
    except OSError:
        return False, ""
    return result.returncode == 0, result.stdout.strip()


def _load_deployments() -> list[dict[str, Any]]:
    if not DEPLOYMENT_FILE.exists():
        return []
    records: list[dict[str, Any]] = []
    with DEPLOYMENT_FILE.open("r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                records.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    return records


def _sha_exists(sha: str) -> bool:
    ok, _ = _run_git(["rev-parse", "--verify", sha])
    return ok


def get_rollback_target(env: str) -> dict[str, Any] | None:
    """Find the previous successful deployment for the given env."""
    if env not in ALLOWED_ENVS:
        raise ValueError(f"env must be one of: {', '.join(sorted(ALLOWED_ENVS))}")
    records = _load_deployments()
    # Walk backwards, skip the most recent, find previous success
    found_current = False
    for record in reversed(records):
        if record.get("env") != env:
            continue
        if record.get("status") != "success":
            continue
        if not found_current:
            found_current = True
            continue  # Skip current deployment
        return record
    return None


def rollback(env: str, target_sha: str | None = None) -> dict[str, Any]:
    """Preview a rollback operation. Does NOT execute git commands."""
    if env not in ALLOWED_ENVS:
        raise ValueError(f"env must be one of: {', '.join(sorted(ALLOWED_ENVS))}")

    # Find current HEAD on the env branch
    ok, current_sha = _run_git(["rev-parse", f"refs/heads/{env}"])
    if not ok:
        return {
            "env": env,
            "action": "rollback",
            "status": "error",
            "error": f"branch '{env}' not found locally",
        }

    if target_sha is None:
        target = get_rollback_target(env)
        if target is None:
            return {
                "env": env,
                "action": "rollback",
                "status": "error",
                "error": "no previous successful deployment found",
            }
        target_sha = target["git_sha"]

    # Validate target SHA
    sha_valid = _sha_exists(target_sha)

    # Generate commands (not executed)
    commands = []
    if sha_valid:
        commands = [
            f"git checkout {env}",
            f"git revert {current_sha[:8]} --no-edit",
            f"git push origin {env}",
            f"git checkout main",
        ]

    return {
        "env": env,
        "action": "rollback",
        "status": "preview",
        "current_sha": current_sha[:8],
        "target_sha": target_sha[:8] if target_sha else None,
        "target_sha_valid": sha_valid,
        "commands": commands,
        "warning": "These commands are NOT executed. Atlas must confirm before running.",
    }


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Standardized rollback tool.")
    subparsers = parser.add_subparsers(dest="command")

    preview_p = subparsers.add_parser("preview", help="Preview rollback target without executing.")
    preview_p.add_argument("--env", required=True, choices=["test", "prod"])

    execute_p = subparsers.add_parser("execute", help="Preview rollback execution plan.")
    execute_p.add_argument("--env", required=True, choices=["test", "prod"])
    execute_p.add_argument("--sha", default=None, help="Target SHA to rollback to.")
    return parser


def main(argv: list[str]) -> int:
    parser = _build_parser()
    args = parser.parse_args(argv[1:])

    if not args.command:
        parser.print_usage(sys.stderr)
        return 1

    try:
        if args.command == "preview":
            result = get_rollback_target(args.env)
            if result is None:
                print(json.dumps({"error": "no previous deployment found"}, indent=2))
                return 1
            print(json.dumps(result, ensure_ascii=False, indent=2))
        elif args.command == "execute":
            result = rollback(args.env, args.sha)
            print(json.dumps(result, ensure_ascii=False, indent=2))
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
