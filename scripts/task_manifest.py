#!/usr/bin/env python3
# v1.0 - Codex task manifest generator and validator

from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime
from pathlib import Path
from typing import Any


SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
MANIFEST_DIR = PROJECT_ROOT / ".atlas" / "manifests"

DEFAULT_DENIED_PATHS = ["vendor/", "node_modules/", ".git/", "docker-compose.prod.yml"]
DEFAULT_DENIED_COMMANDS = ["git push --force", "rm -rf", "DROP TABLE"]


def _now_iso() -> str:
    return datetime.now().astimezone().isoformat(timespec="seconds")


def _normalize_list(items: list[str] | None) -> list[str]:
    if items is None:
        return []

    normalized: list[str] = []
    for item in items:
        value = item.strip()
        if value:
            normalized.append(value)
    return normalized


def _ensure_string_list(name: str, items: Any) -> list[str]:
    if not isinstance(items, list):
        raise ValueError(f"{name} must be a list")

    normalized: list[str] = []
    for index, item in enumerate(items, start=1):
        if not isinstance(item, str):
            raise ValueError(f"{name}[{index}] must be a string")
        value = item.strip()
        if not value:
            raise ValueError(f"{name}[{index}] must not be empty")
        normalized.append(value)
    return normalized


def _manifest_path(task_id: str) -> Path:
    return MANIFEST_DIR / f"{task_id}.json"


def generate_manifest(
    task_id: str,
    allowed_paths: list[str] | None,
    denied_paths: list[str] | None,
    denied_commands: list[str] | None,
) -> dict[str, Any]:
    task_id = task_id.strip()
    if not task_id:
        raise ValueError("task_id is required")

    manifest = {
        "version": "v1.0",
        "task_id": task_id,
        "created_at": _now_iso(),
        "allowed_paths": _normalize_list(allowed_paths),
        "denied_paths": _normalize_list(denied_paths) or DEFAULT_DENIED_PATHS.copy(),
        "denied_commands": _normalize_list(denied_commands) or DEFAULT_DENIED_COMMANDS.copy(),
    }

    MANIFEST_DIR.mkdir(parents=True, exist_ok=True)
    manifest_path = _manifest_path(task_id)
    with manifest_path.open("w", encoding="utf-8") as handle:
        json.dump(manifest, handle, ensure_ascii=False, indent=2, sort_keys=True)
        handle.write("\n")
    return manifest


def validate_manifest(manifest_path: str | Path) -> bool:
    path = Path(manifest_path)
    if not path.exists():
        raise ValueError(f"manifest not found: {path}")

    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        raise ValueError(f"invalid JSON in {path}: {exc}") from exc

    if not isinstance(payload, dict):
        raise ValueError("manifest must be a JSON object")

    required_keys = {"version", "task_id", "created_at", "allowed_paths", "denied_paths", "denied_commands"}
    missing_keys = sorted(required_keys - payload.keys())
    if missing_keys:
        raise ValueError(f"manifest missing keys: {', '.join(missing_keys)}")

    if payload.get("version") != "v1.0":
        raise ValueError("manifest version must be v1.0")
    if not isinstance(payload.get("task_id"), str) or not payload["task_id"].strip():
        raise ValueError("task_id must be a non-empty string")
    if not isinstance(payload.get("created_at"), str) or not payload["created_at"].strip():
        raise ValueError("created_at must be a non-empty string")

    _ensure_string_list("allowed_paths", payload.get("allowed_paths"))
    _ensure_string_list("denied_paths", payload.get("denied_paths"))
    _ensure_string_list("denied_commands", payload.get("denied_commands"))
    return True


def _parse_csv(raw_value: str | None) -> list[str] | None:
    if raw_value is None:
        return None
    return [item for item in raw_value.split(",")]


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Codex task manifest generator.")
    subparsers = parser.add_subparsers(dest="command")

    create_parser = subparsers.add_parser("create", help="Create a task manifest.")
    create_parser.add_argument("--task-id", required=True, help="Task ID such as R0xx.")
    create_parser.add_argument("--allowed-paths", help="Comma-separated allowed paths.")
    create_parser.add_argument("--denied-paths", help="Comma-separated denied paths.")

    validate_parser = subparsers.add_parser("validate", help="Validate a task manifest.")
    validate_parser.add_argument("--manifest", required=True, help="Path to manifest JSON file.")
    return parser


def main(argv: list[str]) -> int:
    parser = _build_parser()
    args = parser.parse_args(argv[1:])

    if not args.command:
        parser.print_usage(sys.stderr)
        return 1

    try:
        if args.command == "create":
            payload: Any = generate_manifest(
                task_id=args.task_id,
                allowed_paths=_parse_csv(args.allowed_paths),
                denied_paths=_parse_csv(args.denied_paths),
                denied_commands=None,
            )
        elif args.command == "validate":
            payload = {
                "manifest": args.manifest,
                "valid": validate_manifest(args.manifest),
            }
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
