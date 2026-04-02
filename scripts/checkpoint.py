#!/usr/bin/env python3
# v1.0 - Append-only checkpoint writer for .atlas/checkpoints.jsonl

from __future__ import annotations

import json
import subprocess
import sys
from datetime import datetime
from pathlib import Path
from typing import Any


SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
ATLAS_DIR = PROJECT_ROOT / ".atlas"
CHECKPOINT_FILE = ATLAS_DIR / "checkpoints.jsonl"


def _now_iso() -> str:
    return datetime.now().astimezone().isoformat(timespec="seconds")


def _run_git(args: list[str]) -> str | None:
    try:
        result = subprocess.run(
            ["git", *args],
            cwd=PROJECT_ROOT,
            check=True,
            capture_output=True,
            text=True,
        )
    except (OSError, subprocess.CalledProcessError):
        return None
    return result.stdout.strip() or None


def _build_record(task_id: str, state: str, message: str) -> dict[str, Any]:
    return {
        "ts": _now_iso(),
        "task_id": task_id,
        "state": state,
        "message": message,
        "git_sha": _run_git(["rev-parse", "HEAD"]),
        "branch": _run_git(["rev-parse", "--abbrev-ref", "HEAD"]),
    }


def write_checkpoint(task_id: str, state: str, message: str = "") -> dict[str, Any]:
    if not task_id:
        raise ValueError("task_id is required")
    if not state:
        raise ValueError("state is required")

    ATLAS_DIR.mkdir(parents=True, exist_ok=True)
    record = _build_record(task_id=task_id, state=state, message=message)
    with CHECKPOINT_FILE.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(record, ensure_ascii=False) + "\n")
    return record


def main(argv: list[str]) -> int:
    if len(argv) < 3:
        print("Usage: python3 scripts/checkpoint.py <task_id> <state> [message]", file=sys.stderr)
        return 1

    task_id = argv[1]
    state = argv[2]
    message = " ".join(argv[3:]) if len(argv) > 3 else ""

    try:
        record = write_checkpoint(task_id=task_id, state=state, message=message)
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    print(json.dumps(record, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
