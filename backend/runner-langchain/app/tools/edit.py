"""Edit tool for file editing (non-overwrite).

Based on Go runner's file_edit.go design.
Performs exact string replacements in files.
"""

import json
import os
from pathlib import Path

from app.tools.registry import get_registry


def _edit(file_path: str, old_string: str, new_string: str) -> str:
    """Perform exact string replacements in a file.

    Args:
        file_path: Path to the file to edit
        old_string: The exact string to replace (must match source text exactly)
        new_string: The replacement string

    Returns:
        JSON string with result
    """
    try:
        path = Path(file_path)

        # Resolve relative paths
        if not path.is_absolute():
            path = Path.cwd() / path

        # Read file content
        if not path.exists():
            return json.dumps({
                "success": False,
                "error": f"File not found: {file_path}",
                "replaced": 0,
            })

        content = path.read_text(encoding="utf-8")

        # Check if old_string exists
        if old_string not in content:
            return json.dumps({
                "success": False,
                "error": "old_string not found in file",
                "replaced": 0,
            })

        # Count replacements before
        replaced = content.count(old_string)

        # Perform replacement
        new_content = content.replace(old_string, new_string)

        # Write back
        path.write_text(new_content, encoding="utf-8")

        return json.dumps({
            "success": True,
            "replaced": replaced,
        })

    except UnicodeDecodeError:
        return json.dumps({
            "success": False,
            "error": "File is not text-readable (binary file?)",
            "replaced": 0,
        })
    except PermissionError:
        return json.dumps({
            "success": False,
            "error": "Permission denied",
            "replaced": 0,
        })
    except Exception as e:
        return json.dumps({
            "success": False,
            "error": str(e),
            "replaced": 0,
        })


def register_tools() -> None:
    """Register edit tool."""
    registry = get_registry()

    registry.register(
        name="edit",
        description="Perform exact string replacements in files. Use this instead of sed or awk commands.",
        schema={
            "type": "object",
            "properties": {
                "file_path": {
                    "type": "string",
                    "description": "Path to the file to edit",
                },
                "old_string": {
                    "type": "string",
                    "description": "The exact string to replace (must match the source text exactly, including whitespace)",
                },
                "new_string": {
                    "type": "string",
                    "description": "The replacement string",
                },
            },
            "required": ["file_path", "old_string", "new_string"],
        },
        handler=lambda **kwargs: _edit(
            file_path=kwargs.get("file_path", ""),
            old_string=kwargs.get("old_string", ""),
            new_string=kwargs.get("new_string", ""),
        ),
    )