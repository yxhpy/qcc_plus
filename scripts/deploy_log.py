#!/usr/bin/env python3
# v1.0 - Deployment audit log tool for .atlas/deployments.jsonl

from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime
from pathlib import Path
from typing import Any


SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
ATLAS_DIR = PROJECT_ROOT / ".atlas"
DEPLOYMENT_FILE = ATLAS_DIR / "deployments.jsonl"

ALLOWED_ENVS = {"test", "prod"}
ALLOWED_ACTORS = {"atlas", "user-confirmation"}
ALLOWED_STATUSES = {"success", "failed", "rollback"}


def _now_iso() -> str:
    return datetime.now().astimezone().isoformat(timespec="seconds")


def _load_deployments() -> list[dict[str, Any]]:
    if not DEPLOYMENT_FILE.exists():
        return []

    records: list[dict[str, Any]] = []
    with DEPLOYMENT_FILE.open("r", encoding="utf-8") as handle:
        for line_number, raw_line in enumerate(handle, start=1):
            line = raw_line.strip()
            if not line:
                continue
            try:
                data = json.loads(line)
            except json.JSONDecodeError as exc:
                raise ValueError(
                    f"invalid JSON in {DEPLOYMENT_FILE} line {line_number}: {exc}"
                ) from exc
            if not isinstance(data, dict):
                raise ValueError(f"invalid deployment object in {DEPLOYMENT_FILE} line {line_number}")
            records.append(data)
    return records


def _validate_env(env: str) -> str:
    if env not in ALLOWED_ENVS:
        raise ValueError(f"env must be one of: {', '.join(sorted(ALLOWED_ENVS))}")
    return env


def _validate_actor(actor: str) -> str:
    if actor not in ALLOWED_ACTORS:
        raise ValueError(f"actor must be one of: {', '.join(sorted(ALLOWED_ACTORS))}")
    return actor


def _validate_status(status: str) -> str:
    if status not in ALLOWED_STATUSES:
        raise ValueError(f"status must be one of: {', '.join(sorted(ALLOWED_STATUSES))}")
    return status


def _validate_limit(limit: int) -> int:
    if limit <= 0:
        raise ValueError("limit must be greater than 0")
    return limit


def _build_record(
    env: str,
    git_sha: str,
    actor: str,
    trigger: str,
    status: str,
    details: str,
) -> dict[str, Any]:
    git_sha = git_sha.strip()
    trigger = trigger.strip()
    if not git_sha:
        raise ValueError("git_sha is required")
    if not trigger:
        raise ValueError("trigger is required")

    return {
        "ts": _now_iso(),
        "env": _validate_env(env),
        "git_sha": git_sha,
        "actor": _validate_actor(actor),
        "trigger": trigger,
        "status": _validate_status(status),
        "details": details,
    }


def log_deployment(
    env: str,
    git_sha: str,
    actor: str,
    trigger: str,
    status: str,
    details: str = "",
) -> dict[str, Any]:
    record = _build_record(
        env=env,
        git_sha=git_sha,
        actor=actor,
        trigger=trigger,
        status=status,
        details=details,
    )

    ATLAS_DIR.mkdir(parents=True, exist_ok=True)
    with DEPLOYMENT_FILE.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(record, ensure_ascii=False) + "\n")
    return record


def query_deployments(env: str | None = None, limit: int = 10) -> list[dict[str, Any]]:
    _validate_limit(limit)
    if env is not None:
        _validate_env(env)

    records = _load_deployments()
    if env is not None:
        records = [record for record in records if record.get("env") == env]
    records.reverse()
    return records[:limit]


def get_current_deployment(env: str) -> dict[str, Any] | None:
    target_env = _validate_env(env)
    for record in reversed(_load_deployments()):
        if record.get("env") != target_env:
            continue
        if record.get("status") in {"success", "rollback"}:
            return record
    return None


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Deployment audit log tool.")
    subparsers = parser.add_subparsers(dest="command")

    log_parser = subparsers.add_parser("log", help="Append a deployment audit record.")
    log_parser.add_argument("--env", required=True, help="Deployment environment: test or prod.")
    log_parser.add_argument("--sha", required=True, dest="git_sha", help="Git SHA for deployment.")
    log_parser.add_argument("--actor", required=True, help="Actor: atlas or user-confirmation.")
    log_parser.add_argument("--trigger", required=True, help="Trigger name for deployment.")
    log_parser.add_argument("--status", required=True, help="Status: success, failed, or rollback.")
    log_parser.add_argument("--details", default="", help="Optional details.")

    query_parser = subparsers.add_parser("query", help="Query recent deployment records.")
    query_parser.add_argument("--env", help="Optional environment filter: test or prod.")
    query_parser.add_argument("--limit", type=int, default=10, help="Number of records to return.")

    current_parser = subparsers.add_parser("current", help="Show current deployed version for env.")
    current_parser.add_argument("--env", required=True, help="Deployment environment: test or prod.")
    return parser


def main(argv: list[str]) -> int:
    parser = _build_parser()
    args = parser.parse_args(argv[1:])

    if not args.command:
        parser.print_usage(sys.stderr)
        return 1

    try:
        if args.command == "log":
            payload: Any = log_deployment(
                env=args.env,
                git_sha=args.git_sha,
                actor=args.actor,
                trigger=args.trigger,
                status=args.status,
                details=args.details,
            )
        elif args.command == "query":
            payload = query_deployments(env=args.env, limit=args.limit)
        elif args.command == "current":
            payload = get_current_deployment(env=args.env)
        else:
            parser.print_usage(sys.stderr)
            return 1
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    print(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
