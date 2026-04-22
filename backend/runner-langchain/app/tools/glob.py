"""Glob tool for file pattern matching.

Based on Go runner's glob.go design.
"""

import json
import os
from pathlib import Path
from typing import Any

from app.tools.registry import get_registry


def _glob(pattern: str, path: str = ".") -> str:
    """Fast file pattern matching tool.

    Args:
        pattern: Glob pattern (e.g., "**/*.py", "*.json")
        path: Base path to search from (optional, defaults to current directory)

    Returns:
        JSON string with matches
    """
    try:
        base_path = Path(path).resolve()

        # Handle recursive patterns
        if "**" in pattern:
            matches = list(base_path.rglob(pattern.replace("**/", "").replace("**", "*")))
        else:
            matches = list(base_path.glob(pattern))

        # Filter to files only, get absolute paths
        abs_matches = []
        for m in matches:
            if m.is_file():
                try:
                    abs_matches.append(str(m.resolve()))
                except (OSError, ValueError):
                    continue

        result = {
            "matches": abs_matches,
            "count": len(abs_matches),
            "pattern": pattern,
            "path": str(base_path),
        }

        return json.dumps(result)

    except Exception as e:
        return json.dumps({"error": str(e), "matches": [], "count": 0})


def _walk_glob(root: str, pattern: str) -> list[str]:
    """Walk directory tree and match files.

    Args:
        root: Root directory
        pattern: Glob pattern to match

    Returns:
        List of matched file paths
    """
    import re as re_module

    matches = []
    regex = re_module.compile(re_module.fnmatch.translate(pattern))

    for dirpath, dirnames, filenames in os.walk(root):
        for filename in filenames:
            if regex.match(filename):
                matches.append(os.path.join(dirpath, filename))

    return matches


def register_tools() -> None:
    """Register glob tool."""
    registry = get_registry()

    registry.register(
        name="glob",
        description="Fast file pattern matching tool. Use this to find files by name patterns (e.g., **/*.py, *.json).",
        schema={
            "type": "object",
            "properties": {
                "pattern": {
                    "type": "string",
                    "description": 'Glob pattern to match files (e.g., "**/*.py", "src/**/*.ts", "*.json")',
                },
                "path": {
                    "type": "string",
                    "description": "Base path to search from (defaults to current directory)",
                },
            },
            "required": ["pattern"],
        },
        handler=lambda **kwargs: _glob(
            pattern=kwargs.get("pattern", ""),
            path=kwargs.get("path", "."),
        ),
    )