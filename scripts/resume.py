#!/usr/bin/env python3
# v1.0 - Resume Codex context from .atlas checkpoints

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path
from typing import Any


SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
CHECKPOINT_FILE = PROJECT_ROOT / ".atlas" / "checkpoints.jsonl"

PROJECT_KNOWLEDGE = [
    "qcc_plus 是面向 Claude Code CLI 的多租户代理服务，当前版本口径为 v1.12.1。",
    "技术栈：Go 1.21+、SQLite / MySQL、React 19、TypeScript、Vite。",
    "默认存储是 SQLite（~/.qccplus/qccplus.db），设置 PROXY_MYSQL_DSN 后可切换 MySQL。",
    "默认只自动创建管理员账号，普通默认账号不再自动创建。",
    "管理端前端路由定义在 frontend/src/App.tsx，后端路由注册在 internal/proxy/handler.go。",
    "默认健康检查方式是 cli，全量健康检查主变量是 PROXY_HEALTH_CHECK_ALL_INTERVAL。",
    "请求日志页面是 RequestLogs.tsx，使用量统计页面是 Usage.tsx。",
    "常用验证命令：go build ./cmd/cccli、bash scripts/build-frontend.sh、go test ./...、cd frontend && npm run build。",
]


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


def _task_history(task_id: str) -> list[dict[str, Any]]:
    return [record for record in _load_checkpoints() if str(record.get("task_id", "")) == task_id]


def _string_value(record: dict[str, Any], key: str) -> str:
    value = record.get(key, "")
    return value if isinstance(value, str) else str(value)


def _state_needs_action(state: str) -> str:
    text = state.strip()
    lowered = text.lower()

    if "开发中" in text or any(token in lowered for token in ("develop", "coding", "implement", "in_progress")):
        return "继续开发"
    if "测试中" in text or any(token in lowered for token in ("test", "qa", "verify")):
        return "推test"
    if "待确认" in text or any(token in lowered for token in ("confirm", "review", "await")):
        return "通知用户"
    return "检查状态"


def _git_sha_exists_in_history(git_sha: str) -> bool:
    if not git_sha:
        return False
    ok, output = _run_git(["branch", "--all", "--contains", git_sha])
    return ok and bool(output.strip())


def _format_history_lines(history: list[dict[str, Any]]) -> list[str]:
    lines: list[str] = []
    for record in history[-5:]:
        timestamp = _string_value(record, "ts") or "unknown-ts"
        state = _string_value(record, "state") or "unknown-state"
        message = _string_value(record, "message") or "无额外说明"
        lines.append(f"- [{timestamp}] {state}: {message}")
    return lines


def resume_task(task_id: str) -> dict[str, Any]:
    if not task_id:
        raise ValueError("task_id is required")

    history = _task_history(task_id)
    if not history:
        raise ValueError(f"checkpoint not found for task_id: {task_id}")

    checkpoint = history[-1]
    git_sha = _string_value(checkpoint, "git_sha")
    branch = _string_value(checkpoint, "branch")
    last_state = _string_value(checkpoint, "state")
    timestamp = _string_value(checkpoint, "ts")
    message = _string_value(checkpoint, "message")

    current_branch_ok, current_branch = _run_git(["rev-parse", "--abbrev-ref", "HEAD"])
    git_sha_exists = _git_sha_exists_in_history(git_sha)
    branch_matches = current_branch_ok and bool(branch) and current_branch == branch

    warnings: list[str] = []
    if not git_sha_exists:
        warnings.append(f"checkpoint git_sha not found in git history: {git_sha or '<empty>'}")
    if not branch:
        warnings.append("checkpoint branch is empty")
    elif not current_branch_ok:
        warnings.append("unable to determine current git branch")
    elif current_branch != branch:
        warnings.append(f"current branch {current_branch} does not match checkpoint branch {branch}")

    return {
        "task_id": task_id,
        "last_state": last_state,
        "git_sha": git_sha,
        "branch": branch,
        "timestamp": timestamp,
        "needs_action": _state_needs_action(last_state),
        "message": message,
        "current_branch": current_branch,
        "git_sha_exists": git_sha_exists,
        "branch_matches_current": branch_matches,
        "resume_blockers": warnings,
    }


def generate_resume_prompt(task_id: str) -> str:
    context = resume_task(task_id)
    history = _task_history(task_id)
    project_knowledge = "\n".join(f"- {item}" for item in PROJECT_KNOWLEDGE)
    progress = "\n".join(_format_history_lines(history)) or "- 暂无历史进度"
    blockers = "\n".join(f"- {item}" for item in context["resume_blockers"]) or "- 无"
    last_message = context["message"] or "无额外说明"

    return (
        f"请恢复并继续处理 qcc_plus 任务 {task_id}。\n\n"
        "项目知识：\n"
        f"{project_knowledge}\n\n"
        "需求上下文：\n"
        f"- task_id: {context['task_id']}\n"
        f"- 上次 checkpoint 状态: {context['last_state']}\n"
        f"- 上次 checkpoint 时间: {context['timestamp']}\n"
        f"- 上次 checkpoint git_sha: {context['git_sha']}\n"
        f"- 上次 checkpoint 分支: {context['branch']}\n"
        f"- 当前分支: {context['current_branch'] or 'unknown'}\n"
        f"- 推荐动作: {context['needs_action']}\n"
        f"- 最新说明: {last_message}\n\n"
        "恢复检查：\n"
        f"- git_sha 在历史中可见: {'是' if context['git_sha_exists'] else '否'}\n"
        f"- 当前分支与 checkpoint 一致: {'是' if context['branch_matches_current'] else '否'}\n"
        f"{blockers}\n\n"
        "上次进度：\n"
        f"{progress}\n\n"
        "继续执行要求：\n"
        "- 如果恢复检查存在阻塞，先处理 git SHA 或分支偏差。\n"
        "- 状态为“开发中”时继续开发并补齐验证。\n"
        "- 状态为“测试中”时准备推 test。\n"
        "- 状态为“待确认”时整理结论并通知用户。\n"
        "- 输出时明确说明当前恢复依据和下一步动作。"
    )


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Resume Codex context from .atlas checkpoints.")
    subparsers = parser.add_subparsers(dest="command")

    status_parser = subparsers.add_parser("status", help="Show resume status for a task.")
    status_parser.add_argument("task_id", help="Task ID to resume.")

    prompt_parser = subparsers.add_parser("prompt", help="Generate a Codex resume prompt.")
    prompt_parser.add_argument("task_id", help="Task ID to resume.")
    return parser


def main(argv: list[str]) -> int:
    parser = _build_parser()
    args = parser.parse_args(argv[1:])

    if not args.command:
        parser.print_usage(sys.stderr)
        return 1

    try:
        if args.command == "status":
            payload: Any = resume_task(args.task_id)
            print(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True))
        elif args.command == "prompt":
            print(generate_resume_prompt(args.task_id))
        else:
            parser.print_usage(sys.stderr)
            return 1
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
