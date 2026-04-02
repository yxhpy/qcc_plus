#!/usr/bin/env python3
# v1.0 - Phase 1 minimal self-introspection engine

from __future__ import annotations

import argparse
import json
import subprocess
from datetime import datetime, timedelta
from pathlib import Path
from typing import Any

from sla_monitor import build_alerts


ACTIVE_SESSION_WINDOW = timedelta(hours=1)
SCRIPT_DIR = Path(__file__).resolve().parent
DEFAULT_PROJECT_ROOT = SCRIPT_DIR.parent


def _now() -> datetime:
    return datetime.now().astimezone()


def _append_jsonl(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(payload, ensure_ascii=False) + "\n")


def _read_session_meta(path: Path) -> dict[str, Any]:
    try:
        with path.open("r", encoding="utf-8") as handle:
            first_line = handle.readline().strip()
    except OSError:
        return {}
    if not first_line:
        return {}
    try:
        payload = json.loads(first_line)
    except json.JSONDecodeError:
        return {}
    return payload.get("payload", {}) if isinstance(payload, dict) else {}


def check_codex_sessions() -> dict[str, Any]:
    sessions_root = Path.home() / ".codex" / "sessions"
    now = _now()
    active_sessions: list[dict[str, Any]] = []

    if sessions_root.exists():
        for session_file in sessions_root.rglob("*.jsonl"):
            try:
                modified_at = datetime.fromtimestamp(session_file.stat().st_mtime, tz=now.tzinfo)
            except OSError:
                continue
            age = now - modified_at
            if age > ACTIVE_SESSION_WINDOW:
                continue
            meta = _read_session_meta(session_file)
            active_sessions.append(
                {
                    "path": str(session_file),
                    "session_id": meta.get("id"),
                    "cwd": meta.get("cwd"),
                    "originator": meta.get("originator"),
                    "modified_at": modified_at.isoformat(timespec="seconds"),
                    "age_minutes": round(age.total_seconds() / 60, 1),
                }
            )

    active_sessions.sort(key=lambda item: item["modified_at"], reverse=True)
    return {
        "sessions_root": str(sessions_root),
        "active_window_minutes": int(ACTIVE_SESSION_WINDOW.total_seconds() / 60),
        "active_count": len(active_sessions),
        "active_sessions": active_sessions,
    }


def check_git_status(project_path: str | Path) -> dict[str, Any]:
    project_dir = Path(project_path).resolve()

    def run_git(args: list[str]) -> str | None:
        try:
            result = subprocess.run(
                ["git", *args],
                cwd=project_dir,
                check=True,
                capture_output=True,
                text=True,
            )
        except (OSError, subprocess.CalledProcessError):
            return None
        return result.stdout.rstrip("\n")

    status_output = run_git(["status", "--porcelain", "--branch"])
    if status_output is None:
        return {
            "project_path": str(project_dir),
            "is_git_repo": False,
            "clean": False,
        }

    lines = [line for line in status_output.splitlines() if line]
    branch_line = lines[0] if lines else ""
    changes = lines[1:] if lines else []

    return {
        "project_path": str(project_dir),
        "is_git_repo": True,
        "clean": not changes,
        "branch": run_git(["rev-parse", "--abbrev-ref", "HEAD"]),
        "git_sha": run_git(["rev-parse", "HEAD"]),
        "summary": branch_line,
        "changes": changes,
    }


def _load_requirements(project_dir: Path) -> list[dict[str, Any]]:
    atlas_dir = project_dir / ".atlas"
    json_path = atlas_dir / "requirements.json"
    jsonl_path = atlas_dir / "requirements.jsonl"

    if json_path.exists():
        with json_path.open("r", encoding="utf-8") as handle:
            data = json.load(handle)
        if isinstance(data, list):
            return [item for item in data if isinstance(item, dict)]
        if isinstance(data, dict):
            requirements = data.get("requirements", [])
            if isinstance(requirements, list):
                return [item for item in requirements if isinstance(item, dict)]
        return []

    if jsonl_path.exists():
        requirements: list[dict[str, Any]] = []
        with jsonl_path.open("r", encoding="utf-8") as handle:
            for raw_line in handle:
                line = raw_line.strip()
                if not line:
                    continue
                item = json.loads(line)
                if isinstance(item, dict):
                    requirements.append(item)
        return requirements

    return []


def check_sla_exceeded(requirements: list[dict[str, Any]]) -> dict[str, Any]:
    alerts: list[dict[str, Any]] = []
    invalid_requirements: list[dict[str, Any]] = []

    for requirement in requirements:
        task_id = requirement.get("task_id")
        state = requirement.get("state")
        entered_at = requirement.get("entered_at")
        priority = requirement.get("priority")

        if not task_id or not state or not entered_at or priority is None:
            invalid_requirements.append(requirement)
            continue

        try:
            alerts.extend(
                build_alerts(
                    task_id=str(task_id),
                    state=str(state),
                    entered_at=str(entered_at),
                    priority=int(priority),
                )
            )
        except (TypeError, ValueError):
            invalid_requirements.append(requirement)

    return {
        "requirements_count": len(requirements),
        "alert_count": len(alerts),
        "alerts": alerts,
        "invalid_requirements": invalid_requirements,
    }


def generate_report(project_path: str | Path) -> dict[str, Any]:
    project_dir = Path(project_path).resolve()
    requirements = _load_requirements(project_dir)
    report = {
        "ts": _now().isoformat(timespec="seconds"),
        "project_path": str(project_dir),
        "codex_sessions": check_codex_sessions(),
        "git_status": check_git_status(project_dir),
        "sla": check_sla_exceeded(requirements),
    }
    _append_jsonl(project_dir / ".atlas" / "introspection.jsonl", report)
    return report


def main() -> int:
    parser = argparse.ArgumentParser(description="Generate a self-introspection report.")
    parser.add_argument("--project", default=str(DEFAULT_PROJECT_ROOT))
    args = parser.parse_args()

    report = generate_report(args.project)
    print(json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
