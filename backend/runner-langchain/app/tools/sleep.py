"""Sleep tool for adding delays between operations.

Based on Go runner's sleep.go design.
"""

import json
import time
from typing import Optional

from app.tools.registry import get_registry


def _sleep(seconds: float) -> str:
    """Wait for a specified duration.

    Args:
        seconds: Number of seconds to sleep (supports fractional values)

    Returns:
        JSON with actual time slept
    """
    if seconds <= 0:
        return json.dumps({"error": "seconds must be positive"})

    if seconds > 300:
        return json.dumps({"error": "seconds cannot exceed 300"})

    start = time.time()
    time.sleep(seconds)
    actual_slept = time.time() - start

    return json.dumps({
        "slept": actual_slept,
        "requested": seconds,
    })


def register_tools() -> None:
    """Register sleep tool."""
    registry = get_registry()

    registry.register(
        name="sleep",
        description="Wait for a specified duration. Use this to add delays between operations.",
        schema={
            "type": "object",
            "properties": {
                "seconds": {
                    "type": "number",
                    "description": "Number of seconds to sleep (supports fractional values like 0.5). Max: 300 seconds.",
                },
            },
            "required": ["seconds"],
        },
        handler=lambda **kwargs: _sleep(kwargs.get("seconds", 0)),
    )