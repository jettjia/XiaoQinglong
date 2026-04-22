"""File operations tools."""

import json
import os
from pathlib import Path
from typing import Any

from app.tools.registry import get_registry


def _file_read(path: str, start_line: int = 1, end_line: int | None = None) -> str:
    """Read a file.

    Args:
        path: Path to the file
        start_line: Line to start reading from (1-indexed)
        end_line: Line to stop reading at (inclusive), None for end of file

    Returns:
        JSON string with file content and metadata
    """
    try:
        p = Path(path)
        if not p.exists():
            return json.dumps({"error": f"File not found: {path}"})

        if not p.is_file():
            return json.dumps({"error": f"Not a file: {path}"})

        content = p.read_text(encoding="utf-8")
        lines = content.splitlines()

        # Convert to 0-indexed
        start = max(0, start_line - 1)
        if end_line is None:
            end = len(lines)
        else:
            end = min(end_line, len(lines))

        selected = lines[start:end]
        selected_content = "\n".join(selected)

        return json.dumps({
            "path": str(p.resolve()),
            "total_lines": len(lines),
            "start_line": start_line,
            "end_line": end,
            "content": selected_content,
        })
    except Exception as e:
        return json.dumps({"error": str(e)})


def _file_write(path: str, content: str, append: bool = False) -> str:
    """Write content to a file.

    Args:
        path: Path to the file
        content: Content to write
        append: If True, append to existing file instead of overwriting

    Returns:
        JSON string with operation result
    """
    try:
        p = Path(path)

        # Create parent directories if needed
        p.parent.mkdir(parents=True, exist_ok=True)

        if append:
            mode = "a"
            action = "appended"
        else:
            mode = "w"
            action = "written"

        p.write_text(content, encoding="utf-8", mode=mode)

        return json.dumps({
            "success": True,
            "path": str(p.resolve()),
            "action": action,
            "bytes": len(content),
        })
    except Exception as e:
        return json.dumps({"error": str(e)})


def _file_list(path: str, pattern: str = "*") -> str:
    """List files in a directory.

    Args:
        path: Directory path
        pattern: Glob pattern for filtering

    Returns:
        JSON string with list of files
    """
    try:
        p = Path(path)
        if not p.exists():
            return json.dumps({"error": f"Directory not found: {path}"})
        if not p.is_dir():
            return json.dumps({"error": f"Not a directory: {path}"})

        files = []
        for item in p.glob(pattern):
            files.append({
                "name": item.name,
                "path": str(item),
                "is_dir": item.is_dir(),
                "size": item.stat().st_size if item.is_file() else 0,
            })

        return json.dumps({
            "path": str(p.resolve()),
            "pattern": pattern,
            "count": len(files),
            "files": files,
        })
    except Exception as e:
        return json.dumps({"error": str(e)})


def register_tools() -> None:
    """Register file operation tools."""
    registry = get_registry()

    registry.register(
        name="file_read",
        description="Read content from a file",
        schema={
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "Path to the file"},
                "start_line": {"type": "integer", "description": "Start line (1-indexed)", "default": 1},
                "end_line": {"type": "integer", "description": "End line (inclusive)", "default": None},
            },
            "required": ["path"],
        },
        handler=_file_read,
    )

    registry.register(
        name="file_write",
        description="Write content to a file",
        schema={
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "Path to the file"},
                "content": {"type": "string", "description": "Content to write"},
                "append": {"type": "boolean", "description": "Append instead of overwrite", "default": False},
            },
            "required": ["path", "content"],
        },
        handler=_file_write,
    )

    registry.register(
        name="file_list",
        description="List files in a directory",
        schema={
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "Directory path"},
                "pattern": {"type": "string", "description": "Glob pattern", "default": "*"},
            },
            "required": ["path"],
        },
        handler=_file_list,
    )
