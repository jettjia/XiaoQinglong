"""Result limiter tool to prevent oversized output.

Based on Go runner's result_limiter.go design.
Wraps tool execution to limit result size.
"""

import json
import logging
import os
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Optional

logger = logging.getLogger(__name__)

# Default max char size: 200KB
DEFAULT_MAX_CHARS = 200 * 1024


@dataclass
class LimitResult:
    """Result of limit check."""
    content: str
    was_limited: bool
    original_size: int
    file_path: Optional[str] = None


class ResultLimiter:
    """Limits tool result sizes by truncating or saving to temp files.

    If a result exceeds max_char_size:
    - Saves to a temp file
    - Returns a message with the file path instead
    """

    def __init__(self, max_chars: int = DEFAULT_MAX_CHARS, temp_dir: Optional[str] = None):
        self.max_chars = max_chars
        self.temp_dir = temp_dir or tempfile.gettempdir()

    def limit_result(self, result: str, tool_name: str = "unknown") -> LimitResult:
        """Check and limit result size.

        Args:
            result: The result string to check
            tool_name: Name of the tool that produced the result

        Returns:
            LimitResult with content and metadata
        """
        if self.max_chars <= 0 or len(result) <= self.max_chars:
            return LimitResult(
                content=result,
                was_limited=False,
                original_size=len(result),
            )

        # Need to limit
        try:
            filename = f"runner_{tool_name}_{os.getpid()}.tmp"
            file_path = Path(self.temp_dir) / filename

            with open(file_path, "w", encoding="utf-8") as f:
                f.write(result)

            truncated_msg = f"\n\n[Result too large ({len(result)} chars), saved to: {file_path}]\n\n"
            return LimitResult(
                content=truncated_msg,
                was_limited=True,
                original_size=len(result),
                file_path=str(file_path),
            )

        except Exception as e:
            logger.warning("[ResultLimiter] Failed to save to temp file: %s", e)
            # Fallback to truncation
            truncated = result[:self.max_chars] + "\n\n[Result truncated - failed to save to temp file]"
            return LimitResult(
                content=truncated,
                was_limited=True,
                original_size=len(result),
            )


# Global limiter instance
_result_limiter: Optional[ResultLimiter] = None


def get_result_limiter() -> ResultLimiter:
    """Get the global result limiter."""
    global _result_limiter
    if _result_limiter is None:
        _result_limiter = ResultLimiter()
    return _result_limiter


def limit_result(result: str, tool_name: str = "unknown") -> LimitResult:
    """Convenience function to limit a result."""
    return get_result_limiter().limit_result(result, tool_name)


def register_tools() -> None:
    """Register result_limiter info tool (read-only)."""
    registry = get_registry()

    registry.register(
        name="result_limiter",
        description="Get information about result size limits and check if a result was limited.",
        schema={
            "type": "object",
            "properties": {
                "action": {
                    "type": "string",
                    "enum": ["info", "check"],
                    "description": "Action: info (get limits), check (check if content exceeds limit)",
                },
                "content": {
                    "type": "string",
                    "description": "Content to check (for check action)",
                },
                "tool_name": {
                    "type": "string",
                    "description": "Tool name (for check action)",
                },
            },
            "required": ["action"],
        },
        handler=lambda **kwargs: _result_limiter_info(
            action=kwargs.get("action", "info"),
            content=kwargs.get("content", ""),
            tool_name=kwargs.get("tool_name", "unknown"),
        ),
    )


def _result_limiter_info(action: str, content: str, tool_name: str) -> str:
    """Handle result_limiter tool calls."""
    limiter = get_result_limiter()

    if action == "info":
        return json.dumps({
            "max_chars": limiter.max_chars,
            "temp_dir": limiter.temp_dir,
        })

    elif action == "check":
        limit_result = limiter.limit_result(content, tool_name)
        return json.dumps({
            "original_size": limit_result.original_size,
            "was_limited": limit_result.was_limited,
            "file_path": limit_result.file_path,
            "current_size": len(limit_result.content),
        })

    return json.dumps({"error": f"Unknown action: {action}"})