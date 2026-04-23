#!/usr/bin/env python3

from __future__ import annotations

from pathlib import Path

MAX_LINE_LENGTH = 160
ROOT = Path(__file__).resolve().parents[1]

INCLUDE_EXTENSIONS = {".css", ".go", ".js", ".proto", ".ts", ".tsx"}
EXCLUDE_DIRS = {".git", "coverage", "dist", "gen", "node_modules"}
EXCLUDE_FILES = {"vite-env.d.ts"}
EXCLUDE_SUFFIXES = {"_test.go"}
EXCLUDE_PATH_PREFIXES = {
    ("web", "src", "lib", "i18n-dictionaries"),
}


def should_check(path: Path) -> bool:
    rel = path.relative_to(ROOT)
    if any(part in EXCLUDE_DIRS for part in rel.parts):
        return False
    if rel.parts[:3] == ("web", "src", "gen"):
        return False
    if any(rel.parts[: len(prefix)] == prefix for prefix in EXCLUDE_PATH_PREFIXES):
        return False
    if path.name in EXCLUDE_FILES:
        return False
    if any(path.name.endswith(suffix) for suffix in EXCLUDE_SUFFIXES):
        return False
    if path.suffix not in INCLUDE_EXTENSIONS:
        return False
    return True


def main() -> int:
    violations: list[str] = []
    for path in sorted(ROOT.rglob("*")):
        if not path.is_file() or not should_check(path):
            continue

        try:
            lines = path.read_text().splitlines()
        except UnicodeDecodeError:
            continue

        rel = path.relative_to(ROOT)
        in_go_import_block = False
        for line_no, line in enumerate(lines, start=1):
            stripped = line.strip()
            if path.suffix == ".go":
                if stripped == "import (":
                    in_go_import_block = True
                elif in_go_import_block and stripped == ")":
                    in_go_import_block = False

                if in_go_import_block or stripped.startswith("import "):
                    continue

            if len(line) <= MAX_LINE_LENGTH:
                continue
            violations.append(
                f"{rel}:{line_no} has {len(line)} characters"
            )

    if not violations:
        print(
            f"All checked source files are within "
            f"{MAX_LINE_LENGTH} characters per line."
        )
        return 0

    print(
        f"Found {len(violations)} line-length violation(s) "
        f"over {MAX_LINE_LENGTH} characters:"
    )
    for violation in violations:
        print(f"  {violation}")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
