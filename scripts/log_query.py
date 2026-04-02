#!/usr/bin/env python3
# v1.0 - Checkpoint log query and state rebuild tool

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any


SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
CHECKPOINT_FILE = PROJECT_ROOT / ".atlas" / "checkpoints.jsonl"


def _load_checkpoints() -> list[dict[str, Any]]:
    if not CHECKPOINT_FILE.exists():
        return []

    records: list[dict[str, Any]] = []
    with CHECKPOINT_FILE.open("r", encoding="utf-8") as handle:
        for line_number, raw_line in enumerate(handle, start=1):
            line = raw_line.strip()
            if not line:
                continue
            try:
                data = json.loads(line)
            except json.JSONDecodeError as exc:
                raise ValueError(
                    f"invalid JSON in {CHECKPOINT_FILE} line {line_number}: {exc}"
                ) from exc
            if not isinstance(data, dict):
                raise ValueError(f"invalid checkpoint object in {CHECKPOINT_FILE} line {line_number}")
            records.append(data)
    return records


def query_task_history(task_id: str) -> list[dict[str, Any]]:
    return [record for record in _load_checkpoints() if record.get("task_id") == task_id]


def query_latest(task_id: str) -> dict[str, Any] | None:
    history = query_task_history(task_id)
    if not history:
        return None
    return history[-1]


def rebuild_state() -> dict[str, dict[str, Any]]:
    latest_by_task: dict[str, dict[str, Any]] = {}
    for record in _load_checkpoints():
        task_id = record.get("task_id")
        if task_id:
            latest_by_task[str(task_id)] = record
    return latest_by_task


def _usage() -> str:
    return (
        "Usage:\n"
        "  python3 scripts/log_query.py history <task_id>\n"
        "  python3 scripts/log_query.py latest <task_id>\n"
        "  python3 scripts/log_query.py rebuild [task_id]"
    )


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print(_usage(), file=sys.stderr)
        return 1

    command = argv[1]
    task_id = argv[2] if len(argv) > 2 else None

    try:
        if command == "history":
            if not task_id:
                print("history requires <task_id>", file=sys.stderr)
                return 1
            payload: Any = query_task_history(task_id)
        elif command == "latest":
            if not task_id:
                print("latest requires <task_id>", file=sys.stderr)
                return 1
            payload = query_latest(task_id)
        elif command == "rebuild":
            payload = rebuild_state()
        else:
            print(_usage(), file=sys.stderr)
            return 1
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    print(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
