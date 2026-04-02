#!/usr/bin/env python3
# v1.0 - Documentation consistency checker

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path
from typing import Any

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent

INTERNAL_DIR = PROJECT_ROOT / "internal"
REGISTRY_FILE = PROJECT_ROOT / "docs" / "modules" / "REGISTRY.md"
API_INDEX_FILE = PROJECT_ROOT / "docs" / "api" / "INDEX.md"
CLAUDE_MD_FILE = PROJECT_ROOT / "CLAUDE.md"
CHANGELOG_FILE = PROJECT_ROOT / "CHANGELOG.md"
HANDLER_FILE = PROJECT_ROOT / "internal" / "proxy" / "handler.go"


def _run_git(args: list[str]) -> str | None:
    try:
        result = subprocess.run(
            ["git", *args],
            cwd=PROJECT_ROOT,
            check=False,
            capture_output=True,
            text=True,
        )
    except OSError:
        return None
    return result.stdout.strip() if result.returncode == 0 else None


def _scan_go_packages() -> set[str]:
    """Scan internal/ for Go package directories."""
    packages: set[str] = set()
    if not INTERNAL_DIR.exists():
        return packages
    for go_file in INTERNAL_DIR.rglob("*.go"):
        if go_file.name.endswith("_test.go"):
            continue
        rel = go_file.relative_to(PROJECT_ROOT)
        pkg = str(rel.parent)
        packages.add(pkg)
    return packages


def _read_file_lines(path: Path) -> list[str]:
    if not path.exists():
        return []
    return path.read_text(encoding="utf-8").splitlines()


def check_registry_sync() -> dict[str, Any]:
    """Check if REGISTRY.md covers all Go packages in internal/."""
    packages = _scan_go_packages()
    if not packages:
        return {
            "check": "registry_sync",
            "status": "skip",
            "reason": "no Go packages found in internal/",
        }

    registry_lines = _read_file_lines(REGISTRY_FILE)
    if not registry_lines:
        return {
            "check": "registry_sync",
            "status": "missing",
            "reason": f"{REGISTRY_FILE.relative_to(PROJECT_ROOT)} not found",
            "packages_found": len(packages),
        }

    # Check which packages are mentioned in registry
    registry_text = "\n".join(registry_lines).lower()
    covered: set[str] = set()
    uncovered: set[str] = set()
    for pkg in sorted(packages):
        # Use last part of path as keyword
        pkg_name = Path(pkg).name.lower()
        if pkg_name in registry_text or pkg.lower() in registry_text:
            covered.add(pkg)
        else:
            uncovered.add(pkg)

    return {
        "check": "registry_sync",
        "status": "pass" if not uncovered else "fail",
        "total_packages": len(packages),
        "covered": len(covered),
        "uncovered_count": len(uncovered),
        "uncovered_samples": sorted(uncovered)[:10],
    }


def check_api_index_sync() -> dict[str, Any]:
    """Check if API INDEX.md covers all registered routes."""
    if not HANDLER_FILE.exists():
        return {
            "check": "api_index_sync",
            "status": "skip",
            "reason": "handler.go not found",
        }

    # Extract HandleFunc routes from handler.go
    handler_text = HANDLER_FILE.read_text(encoding="utf-8")
    route_pattern = re.compile(r'\.HandleFunc\("(/[^"]+)"')
    code_routes: set[str] = set()
    for match in route_pattern.finditer(handler_text):
        code_routes.add(match.group(1))

    if not code_routes:
        return {
            "check": "api_index_sync",
            "status": "skip",
            "reason": "no routes found in handler.go",
        }

    # Read API INDEX
    api_lines = _read_file_lines(API_INDEX_FILE)
    if not api_lines:
        return {
            "check": "api_index_sync",
            "status": "missing",
            "reason": f"{API_INDEX_FILE.relative_to(PROJECT_ROOT)} not found",
            "routes_found": len(code_routes),
        }

    api_text = "\n".join(api_lines).lower()
    covered: set[str] = set()
    uncovered: set[str] = set()
    for route in sorted(code_routes):
        # Normalize for comparison
        route_base = route.rstrip("/")
        if route_base.lower() in api_text:
            covered.add(route)
        else:
            uncovered.add(route)

    return {
        "check": "api_index_sync",
        "status": "pass" if not uncovered else "fail",
        "total_routes": len(code_routes),
        "covered": len(covered),
        "uncovered_count": len(uncovered),
        "uncovered_samples": sorted(uncovered)[:10],
    }


def check_claude_md_freshness() -> dict[str, Any]:
    """Check if CLAUDE.md version matches CHANGELOG.md latest."""
    # Extract version from CHANGELOG.md
    changelog_lines = _read_file_lines(CHANGELOG_FILE)
    changelog_version = None
    for line in changelog_lines:
        match = re.match(r"^##\s+\[?v?(\d+\.\d+\.\d+)", line)
        if match:
            changelog_version = match.group(1)
            break

    if not changelog_version:
        return {
            "check": "claude_md_freshness",
            "status": "skip",
            "reason": "no version found in CHANGELOG.md",
        }

    # Extract version from CLAUDE.md
    claude_lines = _read_file_lines(CLAUDE_MD_FILE)
    claude_version = None
    for line in claude_lines:
        match = re.search(r"v?(\d+\.\d+\.\d+)", line)
        if match:
            claude_version = match.group(1)
            break

    if not claude_version:
        return {
            "check": "claude_md_freshness",
            "status": "missing",
            "reason": "no version found in CLAUDE.md",
            "changelog_version": changelog_version,
        }

    return {
        "check": "claude_md_freshness",
        "status": "pass" if claude_version == changelog_version else "fail",
        "claude_md_version": claude_version,
        "changelog_version": changelog_version,
        "action_needed": "update CLAUDE.md version" if claude_version != changelog_version else None,
    }


def run_all_checks() -> list[dict[str, Any]]:
    return [
        check_registry_sync(),
        check_api_index_sync(),
        check_claude_md_freshness(),
    ]


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Documentation consistency checker.")
    parser.add_argument(
        "--check",
        choices=["registry", "api", "claude"],
        help="Run a specific check only.",
    )
    return parser


def main(argv: list[str]) -> int:
    parser = _build_parser()
    args = parser.parse_args(argv[1:])

    try:
        if args.check == "registry":
            results = [check_registry_sync()]
        elif args.check == "api":
            results = [check_api_index_sync()]
        elif args.check == "claude":
            results = [check_claude_md_freshness()]
        else:
            results = run_all_checks()
    except Exception as exc:
        print(json.dumps({"error": str(exc)}, indent=2), file=sys.stderr)
        return 1

    print(json.dumps(results, ensure_ascii=False, indent=2))

    # Return non-zero if any check failed
    has_failure = any(r.get("status") == "fail" for r in results)
    return 1 if has_failure else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
