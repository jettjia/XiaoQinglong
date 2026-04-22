"""Bash tool for running shell commands."""

import json
import subprocess
from typing import Any

from app.tools.registry import get_registry


def _bash(command: str, timeout: int = 30, cwd: str | None = None) -> str:
    """Execute a bash command.

    Args:
        command: The bash command to execute
        timeout: Timeout in seconds (default 30)
        cwd: Working directory

    Returns:
        JSON string with stdout, stderr, and return code
    """
    try:
        result = subprocess.run(
            command,
            shell=True,
            capture_output=True,
            text=True,
            timeout=timeout,
            cwd=cwd,
        )
        return json.dumps({
            "stdout": result.stdout,
            "stderr": result.stderr,
            "returncode": result.returncode,
        })
    except subprocess.TimeoutExpired:
        return json.dumps({
            "error": f"Command timed out after {timeout} seconds",
            "returncode": -1,
        })
    except Exception as e:
        return json.dumps({
            "error": str(e),
            "returncode": -1,
        })


def register_tools() -> None:
    """Register bash tool."""
    registry = get_registry()
    registry.register(
        name="bash",
        description="Execute a bash command and return the output",
        schema={
            "type": "object",
            "properties": {
                "command": {
                    "type": "string",
                    "description": "The bash command to execute",
                },
                "timeout": {
                    "type": "integer",
                    "description": "Timeout in seconds",
                    "default": 30,
                },
                "cwd": {
                    "type": "string",
                    "description": "Working directory",
                },
            },
            "required": ["command"],
        },
        handler=_bash,
    )
