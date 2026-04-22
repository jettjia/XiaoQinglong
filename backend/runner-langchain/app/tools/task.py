"""Task tools for task tracking.

Based on Go runner's task.go design.
Provides TaskCreate, TaskGet, TaskList, TaskUpdate tools.
"""

import json
import os
import secrets
import threading
import time
from dataclasses import dataclass, asdict
from enum import Enum
from pathlib import Path
from typing import Optional

from app.tools.registry import get_registry
from app.utils.dir import get_base_dir

logger = __name__

TASK_FILE = ".runner_tasks.json"


class TaskStatus(str, Enum):
    PENDING = "pending"
    IN_PROGRESS = "in_progress"
    COMPLETED = "completed"
    CANCELLED = "cancelled"


@dataclass
class Task:
    id: str
    content: str
    status: TaskStatus
    owner: Optional[str] = None
    blocked_by: list[str] = None
    created_at: int = 0
    updated_at: int = 0
    completed_at: Optional[int] = None

    def __post_init__(self):
        if self.blocked_by is None:
            self.blocked_by = []


class TaskStore:
    """Thread-safe task store with disk persistence."""

    _instance: Optional["TaskStore"] = None
    _lock = threading.Lock()

    def __init__(self, task_dir: str = "."):
        self.task_dir = Path(task_dir)
        self.tasks: dict[str, Task] = {}
        self._lock = threading.Lock()
        self._load_from_disk()

    @classmethod
    def get_instance(cls, task_dir: Optional[str] = None) -> "TaskStore":
        """Get singleton task store instance."""
        if cls._instance is None:
            with cls._lock:
                if cls._instance is None:
                    if task_dir is None:
                        task_dir = str(get_base_dir())
                    cls._instance = cls(task_dir)
        return cls._instance

    def _task_file(self) -> Path:
        return self.task_dir / TASK_FILE

    def _load_from_disk(self) -> None:
        """Load tasks from disk."""
        task_file = self._task_file()
        if not task_file.exists():
            return

        try:
            data = json.loads(task_file.read_text())
            for t_data in data:
                t_data["status"] = TaskStatus(t_data["status"])
                task = Task(**t_data)
                self.tasks[task.id] = task
        except (json.JSONDecodeError, KeyError, ValueError):
            pass

    def _save_to_disk(self) -> None:
        """Save tasks to disk."""
        task_file = self._task_file()
        tasks_data = [asdict(t) for t in self.tasks.values()]
        # Convert enums to strings for JSON
        for t_data in tasks_data:
            t_data["status"] = t_data["status"].value
        try:
            task_file.write_text(json.dumps(tasks_data, indent=2))
        except OSError as e:
            logger.warning("Failed to save tasks: %s", e)

    def _generate_id(self) -> str:
        """Generate a unique task ID."""
        return secrets.token_hex(4)

    def create_task(self, content: str, owner: str = "", blocked_by: list[str] = None) -> Task:
        """Create a new task."""
        with self._lock:
            now = int(time.time() * 1000)
            task = Task(
                id=self._generate_id(),
                content=content,
                status=TaskStatus.PENDING,
                owner=owner,
                blocked_by=blocked_by or [],
                created_at=now,
                updated_at=now,
            )
            self.tasks[task.id] = task
            self._save_to_disk()
            return task

    def get_task(self, task_id: str) -> Optional[Task]:
        """Get a task by ID."""
        with self._lock:
            return self.tasks.get(task_id)

    def list_tasks(self, status: Optional[str] = None, owner: Optional[str] = None) -> list[Task]:
        """List tasks with optional filters."""
        with self._lock:
            tasks = list(self.tasks.values())
            if status:
                tasks = [t for t in tasks if t.status.value == status]
            if owner:
                tasks = [t for t in tasks if t.owner == owner]
            return tasks

    def update_task(self, task_id: str, updates: dict) -> Optional[Task]:
        """Update a task."""
        with self._lock:
            task = self.tasks.get(task_id)
            if not task:
                return None

            now = int(time.time() * 1000)
            if "content" in updates:
                task.content = updates["content"]
            if "status" in updates:
                task.status = TaskStatus(updates["status"])
                if task.status == TaskStatus.COMPLETED:
                    task.completed_at = now
            if "owner" in updates:
                task.owner = updates["owner"]
            if "blocked_by" in updates:
                task.blocked_by = updates["blocked_by"]

            task.updated_at = now
            self._save_to_disk()
            return task

    def delete_task(self, task_id: str) -> bool:
        """Delete a task."""
        with self._lock:
            if task_id in self.tasks:
                del self.tasks[task_id]
                self._save_to_disk()
                return True
            return False


# Global store
_task_store: Optional[TaskStore] = None


def get_task_store() -> TaskStore:
    """Get the global task store."""
    global _task_store
    if _task_store is None:
        _task_store = TaskStore.get_instance()
    return _task_store


# ========== Tool Handlers ==========


def _task_create(content: str, owner: str = "", blocked_by: list[str] = None) -> str:
    """Create a new task."""
    try:
        store = get_task_store()
        task = store.create_task(content, owner, blocked_by)
        return json.dumps({
            "id": task.id,
            "content": task.content,
            "status": task.status.value,
            "owner": task.owner,
            "blocked_by": task.blocked_by,
            "created_at": task.created_at,
            "updated_at": task.updated_at,
        })
    except Exception as e:
        return json.dumps({"error": str(e)})


def _task_get(task_id: str) -> str:
    """Get a task by ID."""
    try:
        store = get_task_store()
        task = store.get_task(task_id)
        if not task:
            return json.dumps({"error": f"Task not found: {task_id}"})

        return json.dumps({
            "id": task.id,
            "content": task.content,
            "status": task.status.value,
            "owner": task.owner,
            "blocked_by": task.blocked_by,
            "created_at": task.created_at,
            "updated_at": task.updated_at,
            "completed_at": task.completed_at,
        })
    except Exception as e:
        return json.dumps({"error": str(e)})


def _task_list(status: str = "", owner: str = "") -> str:
    """List all tasks."""
    try:
        store = get_task_store()
        tasks = store.list_tasks(status or None, owner or None)

        return json.dumps({
            "tasks": [
                {
                    "id": t.id,
                    "content": t.content,
                    "status": t.status.value,
                    "owner": t.owner,
                    "blocked_by": t.blocked_by,
                    "created_at": t.created_at,
                    "updated_at": t.updated_at,
                }
                for t in tasks
            ],
            "count": len(tasks),
        })
    except Exception as e:
        return json.dumps({"error": str(e)})


def _task_update(task_id: str, status: str = "", owner: str = "", content: str = "") -> str:
    """Update a task."""
    try:
        store = get_task_store()
        updates = {}
        if status:
            updates["status"] = status
        if owner:
            updates["owner"] = owner
        if content:
            updates["content"] = content

        task = store.update_task(task_id, updates)
        if not task:
            return json.dumps({"error": f"Task not found: {task_id}"})

        return json.dumps({
            "id": task.id,
            "content": task.content,
            "status": task.status.value,
            "owner": task.owner,
            "blocked_by": task.blocked_by,
            "created_at": task.created_at,
            "updated_at": task.updated_at,
        })
    except Exception as e:
        return json.dumps({"error": str(e)})


def register_tools() -> None:
    """Register task tools."""
    registry = get_registry()

    # TaskCreate
    registry.register(
        name="task_create",
        description="Create a new task for tracking progress.",
        schema={
            "type": "object",
            "properties": {
                "content": {
                    "type": "string",
                    "description": "Task description",
                },
                "owner": {
                    "type": "string",
                    "description": "Owner of the task",
                },
                "blocked_by": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Task IDs this task is blocked by",
                },
            },
            "required": ["content"],
        },
        handler=lambda **kwargs: _task_create(
            content=kwargs.get("content", ""),
            owner=kwargs.get("owner", ""),
            blocked_by=kwargs.get("blocked_by"),
        ),
    )

    # TaskGet
    registry.register(
        name="task_get",
        description="Retrieve a task by its ID with full details.",
        schema={
            "type": "object",
            "properties": {
                "task_id": {
                    "type": "string",
                    "description": "Task ID to retrieve",
                },
            },
            "required": ["task_id"],
        },
        handler=lambda **kwargs: _task_get(kwargs.get("task_id", "")),
    )

    # TaskList
    registry.register(
        name="task_list",
        description="List all tasks in the task list.",
        schema={
            "type": "object",
            "properties": {
                "status": {
                    "type": "string",
                    "description": "Filter by status (pending, in_progress, completed, cancelled)",
                },
                "owner": {
                    "type": "string",
                    "description": "Filter by owner",
                },
            },
            "required": [],
        },
        handler=lambda **kwargs: _task_list(
            status=kwargs.get("status", ""),
            owner=kwargs.get("owner", ""),
        ),
    )

    # TaskUpdate
    registry.register(
        name="task_update",
        description="Update a task's status, details, or ownership.",
        schema={
            "type": "object",
            "properties": {
                "task_id": {
                    "type": "string",
                    "description": "Task ID to update",
                },
                "status": {
                    "type": "string",
                    "description": "New status (pending, in_progress, completed, cancelled)",
                },
                "owner": {
                    "type": "string",
                    "description": "New owner",
                },
                "content": {
                    "type": "string",
                    "description": "New task content",
                },
            },
            "required": ["task_id"],
        },
        handler=lambda **kwargs: _task_update(
            task_id=kwargs.get("task_id", ""),
            status=kwargs.get("status", ""),
            owner=kwargs.get("owner", ""),
            content=kwargs.get("content", ""),
        ),
    )