#!/usr/bin/env python3

from __future__ import annotations

import subprocess
import tempfile
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
THRESHOLD = 80.0


def module_path() -> str:
    result = subprocess.run(
        ["go", "list", "-m", "-f", "{{.Path}}"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


def should_skip(path: str) -> bool:
    if path.startswith("cmd/"):
        return True
    if path.startswith("gen/"):
        return True
    if path.endswith("/doc.go") or path == "doc.go":
        return True
    return False


def main() -> int:
    with tempfile.NamedTemporaryFile(
        dir=ROOT, prefix=".coverage-", suffix=".out", delete=False
    ) as tmp:
        profile_path = Path(tmp.name)

    try:
        test_result = subprocess.run(
            ["go", "test", "./...", f"-coverprofile={profile_path}"],
            cwd=ROOT,
            check=False,
        )
        if test_result.returncode != 0:
            return test_result.returncode

        module = module_path()
        covered = defaultdict(int)
        total = defaultdict(int)

        lines = profile_path.read_text().splitlines()
        for line in lines[1:]:
            path_range, stmt_count, hit_count = line.split(" ")
            import_path, _ = path_range.split(":", maxsplit=1)
            if import_path.startswith(module + "/"):
                rel_path = import_path.removeprefix(module + "/")
            else:
                rel_path = import_path

            if should_skip(rel_path):
                continue

            statements = int(stmt_count)
            hits = int(hit_count)
            total[rel_path] += statements
            if hits > 0:
                covered[rel_path] += statements

        failures: list[tuple[str, float]] = []
        for rel_path in sorted(total):
            if total[rel_path] == 0:
                continue
            pct = 100.0 * covered[rel_path] / total[rel_path]
            if pct < THRESHOLD:
                failures.append((rel_path, pct))

        if not failures:
            print(
                "All checked Go files meet the per-file coverage "
                f"threshold of {THRESHOLD:.0f}%."
            )
            return 0

        print(
            "Go per-file coverage threshold failures "
            f"(< {THRESHOLD:.0f}%):"
        )
        for rel_path, pct in failures:
            print(f"  {pct:5.1f}%  {rel_path}")
        return 1
    finally:
        profile_path.unlink(missing_ok=True)
