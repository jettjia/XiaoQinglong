"""TodoWrite tool for simple todo list management.

Based on Go runner's task.go TodoWriteTool design.
Provides add, done, remove, clear actions for todo items.
"""

import json
import threading
from dataclasses import dataclass
from typing import Optional

from app.tools.registry import get_registry


@dataclass
class TodoItem:
    content: str
    done: bool = False


class TodoStore:
    """Thread-safe in-memory todo store."""

    _instance: Optional["TodoStore"] = None
    _lock = threading.Lock()

    def __init__(self):
        self.todos: list[TodoItem] = []
        self._lock = threading.Lock()

    @classmethod
    def get_instance(cls) -> "TodoStore":
        """Get singleton todo store instance."""
        if cls._instance is None:
            with cls._lock:
                if cls._instance is None:
                    cls._instance = cls()
        return cls._instance

    def add(self, content: str) -> None:
        """Add a new todo item."""
        with self._lock:
            self.todos.append(TodoItem(content=content, done=False))

    def done(self, index: int) -> bool:
        """Mark a todo as done by index."""
        with self._lock:
            if 0 <= index < len(self.todos):
                self.todos[index].done = True
                return True
            return False

    def remove(self, index: int) -> bool:
        """Remove a todo by index."""
        with self._lock:
            if 0 <= index < len(self.todos):
                self.todos.pop(index)
                return True
            return False

    def clear(self, todos: list[dict]) -> None:
        """Replace all todos with new list."""
        with self._lock:
            self.todos = [
                TodoItem(content=t.get("content", ""), done=t.get("done", False))
                for t in todos
            ]

    def list_all(self) -> list[dict]:
        """List all todos."""
        with self._lock:
            return [
                {"content": t.content, "done": t.done}
                for t in self.todos
            ]


# Global store
_todo_store: Optional[TodoStore] = None


def get_todo_store() -> TodoStore:
    """Get the global todo store."""
    global _todo_store
    if _todo_store is None:
        _todo_store = TodoStore.get_instance()
    return _todo_store


def _todo_write(action: str, content: str = "", index: int = -1, todos: list[dict] = None) -> str:
    """Manage todo list.

    Actions:
    - add: Add a new todo
    - done: Mark todo as complete
    - remove: Delete a todo
    - clear: Replace all todos
    """
    try:
        store = get_todo_store()

        if action == "add":
            if not content:
                return json.dumps({"error": "content required for add action"})
            store.add(content)

        elif action == "done":
            if index < 0:
                return json.dumps({"error": "index required for done action"})
            if not store.done(index):
                return json.dumps({"error": f"Invalid index: {index}"})

        elif action == "remove":
            if index < 0:
                return json.dumps({"error": "index required for remove action"})
            if not store.remove(index):
                return json.dumps({"error": f"Invalid index: {index}"})

        elif action == "clear":
            store.clear(todos or [])

        elif action == "list":
            # Just list, no modification
            pass

        else:
            return json.dumps({"error": f"Unknown action: {action}"})

        # Return current todo list
        all_todos = store.list_all()
        return json.dumps({
            "todos": all_todos,
            "count": len(all_todos),
        })

    except Exception as e:
        return json.dumps({"error": str(e)})


def register_tools() -> None:
    """Register todo_write tool."""
    registry = get_registry()

    registry.register(
        name="todo_write",
        description="Create and manage a todo list for tracking progress.",
        schema={
            "type": "object",
            "properties": {
                "action": {
                    "type": "string",
                    "enum": ["add", "done", "remove", "clear"],
                    "description": "Action: add (add todo), done (mark complete), remove (delete), clear (replace all)",
                },
                "content": {
                    "type": "string",
                    "description": "Todo content (for add action)",
                },
                "index": {
                    "type": "integer",
                    "description": "Todo index (for done/remove actions, 0-indexed)",
                },
                "todos": {
                    "type": "array",
                    "description": "Full todo list (for clear action)",
                    "items": {
                        "type": "object",
                        "properties": {
                            "content": {"type": "string"},
                            "done": {"type": "boolean"},
                        },
                    },
                },
            },
            "required": ["action"],
        },
        handler=lambda **kwargs: _todo_write(
            action=kwargs.get("action", ""),
            content=kwargs.get("content", ""),
            index=kwargs.get("index", -1),
            todos=kwargs.get("todos"),
        ),
    )