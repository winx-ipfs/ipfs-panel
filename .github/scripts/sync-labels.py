#!/usr/bin/env python3
"""Create or update labels from .github/labels.json. Does not delete extras."""

import json
import subprocess
import sys
from pathlib import Path


def run(args: list[str]) -> str:
    result = subprocess.run(args, check=True, text=True, capture_output=True)
    return result.stdout


def main() -> int:
    root = Path(__file__).resolve().parents[2]
    desired = json.loads((root / ".github" / "labels.json").read_text(encoding="utf-8"))
    existing = {
        item["name"]
        for item in json.loads(run(["gh", "label", "list", "--limit", "200", "--json", "name"]))
    }
    for label in desired:
        name = label["name"]
        color = label["color"].removeprefix("#")
        description = label.get("description", "")
        if name in existing:
            run(["gh", "label", "edit", name, "--color", color, "--description", description])
            print(f"updated {name}")
        else:
            run(
                [
                    "gh",
                    "label",
                    "create",
                    name,
                    "--color",
                    color,
                    "--description",
                    description,
                ]
            )
            print(f"created {name}")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except subprocess.CalledProcessError as exc:
        sys.stderr.write(exc.stderr or str(exc))
        sys.exit(exc.returncode)
