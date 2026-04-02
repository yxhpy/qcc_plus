#!/usr/bin/env python3
# v1.0 - SLA alert generator for requirement states

from __future__ import annotations

import argparse
import json
from datetime import datetime
from typing import Any


SLA_HOURS = {
    1: 6,
    2: 12,
    3: 48,
}


def _now() -> datetime:
    return datetime.now().astimezone()


def _parse_datetime(value: str) -> datetime:
    normalized = value.replace("Z", "+00:00")
    parsed = datetime.fromisoformat(normalized)
    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=_now().tzinfo)
    return parsed.astimezone()


def build_alerts(
    task_id: str,
    state: str,
    entered_at: str,
    priority: int,
    now: datetime | None = None,
) -> list[dict[str, Any]]:
    if priority not in SLA_HOURS:
        raise ValueError("priority must be 1, 2, or 3")

    checked_at = now.astimezone() if now else _now()
    entered_dt = _parse_datetime(entered_at)
    elapsed_hours = round((checked_at - entered_dt).total_seconds() / 3600, 2)
    sla_hours = SLA_HOURS[priority]

    if elapsed_hours < sla_hours:
        return []

    severity = "critical" if elapsed_hours >= sla_hours * 2 else "warning"
    status = "严重阻塞" if severity == "critical" else "已超时"
    message = (
        f"任务 {task_id} 当前状态为 {state}，已持续 {elapsed_hours} 小时，"
        f"超过 P{priority} SLA {sla_hours} 小时"
    )

    return [
        {
            "task_id": task_id,
            "state": state,
            "priority": priority,
            "entered_at": entered_dt.isoformat(timespec="seconds"),
            "checked_at": checked_at.isoformat(timespec="seconds"),
            "elapsed_hours": elapsed_hours,
            "sla_hours": sla_hours,
            "status": status,
            "severity": severity,
            "message": message,
        }
    ]


def main() -> int:
    parser = argparse.ArgumentParser(description="Check whether a requirement exceeded SLA.")
    parser.add_argument("--task-id", required=True)
    parser.add_argument("--state", required=True)
    parser.add_argument("--entered-at", required=True)
    parser.add_argument("--priority", required=True, type=int)
    args = parser.parse_args()

    alerts = build_alerts(
        task_id=args.task_id,
        state=args.state,
        entered_at=args.entered_at,
        priority=args.priority,
    )
    print(json.dumps(alerts, ensure_ascii=False, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
