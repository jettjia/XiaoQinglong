"""Grep tool for content search.

Based on Go runner's grep.go design.
"""

import json
import os
import re
import subprocess
from pathlib import Path
from typing import Any

from app.tools.registry import get_registry


def _grep(
    pattern: str,
    path: str,
    glob: str = None,
    context: int = 0,
    case_sensitive: bool = True,
    head: int = 0,
    invert: bool = False,
) -> str:
    """Content search tool using regex.

    Args:
        pattern: Regular expression pattern to search for
        path: Path to search in (file or directory)
        glob: Only search files matching glob pattern (e.g., "*.py")
        context: Number of context lines before/after match
        case_sensitive: Case sensitive search (default true)
        head: Limit number of results
        invert: Invert match (like grep -v)

    Returns:
        JSON string with matches
    """
    try:
        base_path = Path(path).resolve()

        # Try using ripgrep (rg) first for better performance
        try:
            result = _grep_ripgrep(
                pattern=pattern,
                path=str(base_path),
                glob=glob,
                context=context,
                case_sensitive=case_sensitive,
                head=head,
                invert=invert,
            )
            return result
        except FileNotFoundError:
            # Fall back to Python implementation if rg not available
            pass

        # Python fallback implementation
        matches = _grep_python(
            pattern=pattern,
            path=str(base_path),
            glob=glob,
            case_sensitive=case_sensitive,
            head=head,
            invert=invert,
        )

        return json.dumps({
            "matches": matches,
            "count": len(matches),
            "pattern": pattern,
            "path": str(base_path),
        })

    except Exception as e:
        return json.dumps({"error": str(e), "matches": [], "count": 0})


def _grep_ripgrep(
    pattern: str,
    path: str,
    glob: str = None,
    context: int = 0,
    case_sensitive: bool = True,
    head: int = 0,
    invert: bool = False,
) -> str:
    """Use ripgrep for searching."""
    args = ["rg", "-n", "--json"]

    if context > 0:
        args.extend(["-C", str(context)])

    if case_sensitive:
        args.append("-s")
    else:
        args.append("-i")

    if invert:
        args.append("-v")

    if head > 0:
        args.extend(["-m", str(head)])

    if glob:
        args.extend(["-g", glob])

    args.extend([pattern, path])

    result = subprocess.run(
        args,
        capture_output=True,
        text=True,
    )

    matches = []
    for line in result.stdout.splitlines():
        if not line.strip():
            continue
        try:
            rg_result = json.loads(line)
            if rg_result.get("type") == "match":
                matches.append({
                    "file": rg_result.get("data", {}).get("path", {}).get("text", ""),
                    "line": rg_result.get("data", {}).get("line_number", 0),
                    "text": rg_result.get("data", {}).get("lines", {}).get("text", "").rstrip("\n"),
                })
        except json.JSONDecodeError:
            continue

    # ripgrep returns exit code 1 when no matches, which is not an error
    return json.dumps({
        "matches": matches,
        "count": len(matches),
        "pattern": pattern,
        "path": path,
    })


def _grep_python(
    pattern: str,
    path: str,
    glob: str = None,
    case_sensitive: bool = True,
    head: int = 0,
    invert: bool = False,
) -> list[dict]:
    """Pure Python grep implementation as fallback."""
    matches = []

    # Compile regex
    flags = 0 if case_sensitive else re.IGNORECASE
    try:
        regex = re.compile(pattern, flags)
    except re.error:
        return []

    # Determine search scope
    search_path = Path(path)
    if search_path.is_file():
        files_to_search = [search_path]
    else:
        files_to_search = []
        if glob:
            files_to_search = list(search_path.rglob(glob))
        else:
            # Search all files in directory
            for root, dirs, files in os.walk(search_path):
                # Skip common exclusions
                dirs[:] = [d for d in dirs if d not in (".git", ".venv", "node_modules", "__pycache__")]
                for filename in files:
                    if filename.startswith("."):
                        continue
                    files_to_search.append(Path(root) / filename)

    for file_path in files_to_search:
        if not file_path.is_file():
            continue

        try:
            with open(file_path, "r", encoding="utf-8", errors="ignore") as f:
                for line_num, line in enumerate(f, 1):
                    line = line.rstrip("\n")
                    has_match = bool(regex.search(line))

                    if invert:
                        has_match = not has_match

                    if has_match:
                        matches.append({
                            "file": str(file_path),
                            "line": line_num,
                            "text": line,
                        })

                        if head > 0 and len(matches) >= head:
                            return matches

        except (OSError, UnicodeDecodeError):
            continue

    return matches


def register_tools() -> None:
    """Register grep tool."""
    registry = get_registry()

    registry.register(
        name="grep",
        description="Content search tool using regex. Searches files for regex patterns.",
        schema={
            "type": "object",
            "properties": {
                "pattern": {
                    "type": "string",
                    "description": "Regular expression pattern to search for",
                },
                "path": {
                    "type": "string",
                    "description": "Path to search in (file or directory)",
                },
                "glob": {
                    "type": "string",
                    "description": 'Only search files matching glob pattern (e.g., "*.py")',
                },
                "context": {
                    "type": "integer",
                    "description": "Number of context lines before/after match",
                },
                "case_sensitive": {
                    "type": "boolean",
                    "description": "Case sensitive search (default true)",
                },
                "head": {
                    "type": "integer",
                    "description": "Limit number of results",
                },
                "invert": {
                    "type": "boolean",
                    "description": "Invert match (like grep -v)",
                },
            },
            "required": ["pattern", "path"],
        },
        handler=lambda **kwargs: _grep(
            pattern=kwargs.get("pattern", ""),
            path=kwargs.get("path", "."),
            glob=kwargs.get("glob"),
            context=kwargs.get("context", 0),
            case_sensitive=kwargs.get("case_sensitive", True),
            head=kwargs.get("head", 0),
            invert=kwargs.get("invert", False),
        ),
    )