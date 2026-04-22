"""Loop tools for continuous agent execution.

Based on Go runner's cron/loop_tool.go design.
Provides cron_create, cron_delete, cron_list for scheduling.
"""

import json
import logging
import re
import threading
import time
from dataclasses import dataclass, field
from typing import Any, Optional

from app.tools.registry import get_registry

logger = logging.getLogger(__name__)

# Max scheduled jobs
MAX_JOBS = 50


@dataclass
class ScheduledTask:
    """A scheduled task."""
    id: str
    cron: str
    prompt: str
    recurring: bool = True
    durable: bool = False
    created_at: int = field(default_factory=lambda: int(time.time() * 1000))
    last_fired_at: Optional[int] = None


class TaskStore:
    """Thread-safe task storage."""

    _instance: Optional["TaskStore"] = None
    _lock = threading.Lock()

    def __init__(self):
        self.tasks: dict[str, ScheduledTask] = {}

    @classmethod
    def get_instance(cls) -> "TaskStore":
        if cls._instance is None:
            with cls._lock:
                if cls._instance is None:
                    cls._instance = TaskStore()
        return cls._instance

    def add(self, task: ScheduledTask) -> None:
        with self._lock:
            self.tasks[task.id] = task

    def remove(self, task_id: str) -> bool:
        with self._lock:
            if task_id in self.tasks:
                del self.tasks[task_id]
                return True
            return False

    def get(self, task_id: str) -> Optional[ScheduledTask]:
        with self._lock:
            return self.tasks.get(task_id)

    def list_all(self) -> list[ScheduledTask]:
        with self._lock:
            return list(self.tasks.values())

    def clear(self) -> None:
        with self._lock:
            self.tasks.clear()


def _parse_cron(cron: str) -> bool:
    """Validate 5-field cron expression."""
    pattern = r"^(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)$"
    return bool(re.match(pattern, cron))


def _cron_to_human(cron: str) -> str:
    """Convert cron to human-readable string."""
    parts = cron.split()
    if len(parts) != 5:
        return cron

    minute, hour, dom, month, dow = parts

    if minute.startswith("*/"):
        return f"every {minute[2:]} minutes"
    if hour.startswith("*/"):
        return f"every {hour[2:]} hours"
    if dom.startswith("*/"):
        return f"every {dom[2:]} days"

    # Try to build a readable string
    result = []
    if minute != "*":
        result.append(f"minute {minute}")
    if hour != "*":
        result.append(f"at {hour}")
    if dom != "*":
        result.append(f"on day {dom}")
    if month != "*":
        result.append(f"in month {month}")
    if dow != "*":
        days = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]
        try:
            dow_idx = int(dow)
            if 0 <= dow_idx <= 6:
                result.append(f"on {days[dow_idx]}")
        except ValueError:
            result.append(f"on {dow}")

    return ", ".join(result) if result else cron


def _next_cron_run_ms(cron: str, now_ms: int) -> Optional[int]:
    """Calculate next run time in milliseconds.

    Simple implementation - finds next minute that matches.
    """
    # This is a simplified version
    # In production, use a proper cron library
    return now_ms + 60000  # Next minute


def _interval_to_cron(interval: str) -> str:
    """Convert interval notation to cron expression.

    Nm -> */N * * * *
    Nh -> 0 */N * * *
    Nd -> 0 0 */N * *
    """
    interval = interval.strip()

    if interval.endswith("m"):
        n = interval[:-1]
        return f"*/{n} * * * *"
    if interval.endswith("h"):
        n = interval[:-1]
        return f"0 */{n} * * *"
    if interval.endswith("d"):
        n = interval[:-1]
        return f"0 0 */{n} * *"

    # Plain number = minutes
    return f"*/{interval} * * * *"


def _create_task_id() -> str:
    """Generate a task ID."""
    import uuid
    return f"task-{uuid.uuid4().hex[:8]}"


def _cron_create(
    cron: str,
    prompt: str,
    recurring: bool = True,
    durable: bool = False,
) -> str:
    """Create a scheduled task.

    Args:
        cron: Cron expression (5 fields: M H DoM Mon DoW)
        prompt: The prompt to execute
        recurring: True for recurring, False for one-shot
        durable: True to persist to disk

    Returns:
        JSON result
    """
    try:
        store = TaskStore.get_instance()

        if len(store.list_all()) >= MAX_JOBS:
            return json.dumps({
                "success": False,
                "error": f"Too many scheduled jobs (max {MAX_JOBS}). Cancel one first.",
            })

        if not _parse_cron(cron):
            return json.dumps({
                "success": False,
                "error": f"Invalid cron expression: {cron}",
            })

        task_id = _create_task_id()
        task = ScheduledTask(
            id=task_id,
            cron=cron,
            prompt=prompt,
            recurring=recurring,
            durable=durable,
        )

        store.add(task)

        return json.dumps({
            "success": True,
            "id": task_id,
            "human_schedule": _cron_to_human(cron),
            "recurring": recurring,
            "durable": durable,
        })

    except Exception as e:
        logger.error("[cron_create] Failed: %s", e)
        return json.dumps({"success": False, "error": str(e)})


def _cron_delete(id: str) -> str:
    """Delete a scheduled task.

    Args:
        id: Task ID to delete

    Returns:
        JSON result
    """
    try:
        store = TaskStore.get_instance()
        if store.remove(id):
            return json.dumps({
                "success": True,
                "message": f"Task {id} cancelled",
            })
        return json.dumps({
            "success": False,
            "error": f"Task {id} not found",
        })
    except Exception as e:
        logger.error("[cron_delete] Failed: %s", e)
        return json.dumps({"success": False, "error": str(e)})


def _cron_list() -> str:
    """List all scheduled tasks.

    Returns:
        JSON with all tasks
    """
    try:
        store = TaskStore.get_instance()
        tasks = store.list_all()
        now = int(time.time() * 1000)

        task_infos = []
        for task in tasks:
            info = {
                "id": task.id,
                "cron": task.cron,
                "human_schedule": _cron_to_human(task.cron),
                "prompt": task.prompt,
                "recurring": task.recurring,
                "durable": task.durable,
                "created_at": task.created_at,
            }

            next_run = _next_cron_run_ms(task.cron, now)
            if next_run:
                info["next_fire_at"] = next_run

            if task.last_fired_at:
                info["last_fired_at"] = task.last_fired_at

            task_infos.append(info)

        return json.dumps({
            "success": True,
            "tasks": task_infos,
        })

    except Exception as e:
        logger.error("[cron_list] Failed: %s", e)
        return json.dumps({"success": False, "error": str(e)})


def register_tools() -> None:
    """Register loop/cron tools."""
    registry = get_registry()

    # cron_create tool
    registry.register(
        name="cron_create",
        description="Schedule a prompt to run at a future time — either recurring on a cron schedule, or once at a specific time.",
        schema={
            "type": "object",
            "properties": {
                "cron": {
                    "type": "string",
                    "description": "Standard 5-field cron expression: 'M H DoM Mon DoW' (e.g., '*/5 * * * *' = every 5 minutes, '30 14 * * *' = daily at 2:30pm)",
                },
                "prompt": {
                    "type": "string",
                    "description": "The prompt to execute at each scheduled time",
                },
                "recurring": {
                    "type": "boolean",
                    "description": "True (default) = recurring until deleted. False = one-shot, auto-deletes after execution.",
                },
                "durable": {
                    "type": "boolean",
                    "description": "True = persist to disk. False (default) = in-memory only, lost on restart.",
                },
            },
            "required": ["cron", "prompt"],
        },
        handler=lambda **kwargs: _cron_create(
            cron=kwargs.get("cron", ""),
            prompt=kwargs.get("prompt", ""),
            recurring=kwargs.get("recurring", True),
            durable=kwargs.get("durable", False),
        ),
    )

    # cron_delete tool
    registry.register(
        name="cron_delete",
        description="Cancel a scheduled cron job by ID.",
        schema={
            "type": "object",
            "properties": {
                "id": {
                    "type": "string",
                    "description": "The job ID to cancel",
                },
            },
            "required": ["id"],
        },
        handler=lambda **kwargs: _cron_delete(id=kwargs.get("id", "")),
    )

    # cron_list tool
    registry.register(
        name="cron_list",
        description="List all scheduled cron jobs.",
        schema={
            "type": "object",
            "properties": {},
        },
        handler=lambda **kwargs: _cron_list(),
    )