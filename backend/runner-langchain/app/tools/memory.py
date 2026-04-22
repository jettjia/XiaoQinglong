"""Memory tool for persistent curated memory.

Provides bounded, file-backed memory that persists across sessions:
- MEMORY.md: agent's personal notes and observations
- USER.md: what the agent knows about the user

Based on hermes-agent's memory_tool.py design.
"""

import fcntl
import json
import logging
import os
import re
from contextlib import contextmanager
from pathlib import Path
from typing import Any

from app.tools.registry import get_registry

logger = logging.getLogger(__name__)

ENTRY_DELIMITER = "\n§\n"

# Memory directory (follows xqldir design)
DEFAULT_MEMORY_DIR = Path.home() / ".xiaoqinglong" / "memory"

# Character limits
DEFAULT_MEMORY_LIMIT = 2200
DEFAULT_USER_LIMIT = 1375

# Threat patterns for content scanning
_THREAT_PATTERNS = [
    (r'ignore\s+(previous|all|above|prior)\s+instructions', "prompt_injection"),
    (r'you\s+are\s+now\s+', "role_hijack"),
    (r'do\s+not\s+tell\s+the\s+user', "deception_hide"),
    (r'system\s+prompt\s+override', "sys_prompt_override"),
    (r'disregard\s+(your|all|any)\s+(instructions|rules|guidelines)', "disregard_rules"),
]

_INVISIBLE_CHARS = {
    '​', '‌', '‍', '⁠', '﻿',
    '‪', '‫', '‬', '‭', '‮',
}


def _scan_content(content: str) -> str | None:
    """Scan content for injection/exfiltration patterns. Returns error message if blocked."""
    for char in _INVISIBLE_CHARS:
        if char in content:
            return f"Blocked: invisible unicode U+{ord(char):04X}"

    for pattern, pid in _THREAT_PATTERNS:
        if re.search(pattern, content, re.IGNORECASE):
            return f"Blocked: threat pattern '{pid}'"

    return None


def get_memory_dir() -> Path:
    """Get the memory directory path."""
    return DEFAULT_MEMORY_DIR


@contextmanager
def _file_lock(path: Path):
    """Acquire exclusive file lock for read-modify-write safety."""
    lock_path = path.with_suffix(path.suffix + ".lock")
    lock_path.parent.mkdir(parents=True, exist_ok=True)
    fd = open(lock_path, "w")
    try:
        fcntl.flock(fd, fcntl.LOCK_EX)
        yield
    finally:
        fcntl.flock(fd, fcntl.LOCK_UN)
        fd.close()


class MemoryStore:
    """Bounded curated memory with file persistence."""

    def __init__(
        self,
        memory_char_limit: int = DEFAULT_MEMORY_LIMIT,
        user_char_limit: int = DEFAULT_USER_LIMIT,
    ):
        self.memory_entries: list[str] = []
        self.user_entries: list[str] = []
        self.memory_char_limit = memory_char_limit
        self.user_char_limit = user_char_limit
        self._system_prompt_snapshot: dict[str, str] = {"memory": "", "user": ""}

    def load_from_disk(self) -> None:
        """Load entries from MEMORY.md and USER.md."""
        mem_dir = get_memory_dir()
        mem_dir.mkdir(parents=True, exist_ok=True)

        self.memory_entries = self._read_file(mem_dir / "MEMORY.md")
        self.user_entries = self._read_file(mem_dir / "USER.md")

        # Deduplicate
        self.memory_entries = list(dict.fromkeys(self.memory_entries))
        self.user_entries = list(dict.fromkeys(self.user_entries))

        self._system_prompt_snapshot = {
            "memory": self._render_block("memory", self.memory_entries),
            "user": self._render_block("user", self.user_entries),
        }

    def _read_file(self, path: Path) -> list[str]:
        """Read entries from a memory file."""
        if not path.exists():
            return []

        try:
            content = path.read_text(encoding="utf-8")
            entries = content.split(ENTRY_DELIMITER)
            return [e.strip() for e in entries if e.strip()]
        except Exception as e:
            logger.error(f"Failed to read memory file {path}: {e}")
            return []

    def _write_file(self, path: Path, entries: list[str]) -> None:
        """Write entries to a memory file."""
        try:
            content = ENTRY_DELIMITER.join(entries)
            path.write_text(content, encoding="utf-8")
        except Exception as e:
            logger.error(f"Failed to write memory file {path}: {e}")

    def _path_for(self, target: str) -> Path:
        """Get path for target memory file."""
        mem_dir = get_memory_dir()
        return mem_dir / f"{target.upper()}.md"

    def _entries_for(self, target: str) -> list[str]:
        """Get entries for target."""
        return self.user_entries if target == "user" else self.memory_entries

    def _set_entries(self, target: str, entries: list[str]) -> None:
        """Set entries for target."""
        if target == "user":
            self.user_entries = entries
        else:
            self.memory_entries = entries

    def _char_count(self, target: str) -> int:
        """Get character count for target."""
        entries = self._entries_for(target)
        if not entries:
            return 0
        return len(ENTRY_DELIMITER.join(entries))

    def _char_limit(self, target: str) -> int:
        """Get character limit for target."""
        return self.user_char_limit if target == "user" else self.memory_char_limit

    def _render_block(self, target: str, entries: list[str]) -> str:
        """Render memory block for system prompt."""
        if not entries:
            return f"[{target.upper()} memory: empty]"

        header = f"[{target.upper()} MEMORY — {len(entries)} entries, {self._char_count(target)}/{self._char_limit(target)} chars]"
        content = ENTRY_DELIMITER.join(entries)
        return f"{header}]\n{content}\n[/{target.upper()} MEMORY]"

    def add(self, target: str, content: str) -> dict[str, Any]:
        """Add a new entry."""
        content = content.strip()
        if not content:
            return {"success": False, "error": "Content cannot be empty."}

        # Scan for threats
        scan_error = _scan_content(content)
        if scan_error:
            return {"success": False, "error": scan_error}

        with _file_lock(self._path_for(target)):
            entries = self._entries_for(target)
            limit = self._char_limit(target)

            # Check duplicate
            if content in entries:
                return {"success": False, "error": "Entry already exists."}

            # Check limit
            new_entries = entries + [content]
            new_total = len(ENTRY_DELIMITER.join(new_entries))

            if new_total > limit:
                current = self._char_count(target)
                return {
                    "success": False,
                    "error": f"Memory at {current}/{limit} chars. Adding this ({len(content)} chars) would exceed limit.",
                }

            entries.append(content)
            self._set_entries(target, entries)
            self._write_file(self._path_for(target), entries)

        return {"success": True, "message": "Entry added."}

    def replace(self, target: str, old_text: str, new_content: str) -> dict[str, Any]:
        """Replace entry containing old_text with new_content."""
        old_text = old_text.strip()
        new_content = new_content.strip()

        if not old_text:
            return {"success": False, "error": "old_text cannot be empty."}
        if not new_content:
            return {"success": False, "error": "new_content cannot be empty."}

        # Scan new content
        scan_error = _scan_content(new_content)
        if scan_error:
            return {"success": False, "error": scan_error}

        with _file_lock(self._path_for(target)):
            entries = self._entries_for(target)

            # Find entry to replace
            found_idx = None
            for i, entry in enumerate(entries):
                if old_text in entry:
                    found_idx = i
                    break

            if found_idx is None:
                return {"success": False, "error": f"No entry found containing '{old_text}'"}

            # Replace
            entries[found_idx] = new_content
            self._set_entries(target, entries)
            self._write_file(self._path_for(target), entries)

        return {"success": True, "message": "Entry replaced."}

    def remove(self, target: str, text: str) -> dict[str, Any]:
        """Remove entry containing text."""
        text = text.strip()
        if not text:
            return {"success": False, "error": "text cannot be empty."}

        with _file_lock(self._path_for(target)):
            entries = self._entries_for(target)

            # Find and remove
            new_entries = [e for e in entries if text not in e]

            if len(new_entries) == len(entries):
                return {"success": False, "error": f"No entry found containing '{text}'"}

            self._set_entries(target, new_entries)
            self._write_file(self._path_for(target), new_entries)

        return {"success": True, "message": "Entry removed."}

    def read(self, target: str = "all") -> dict[str, Any]:
        """Read memory entries."""
        if target == "all":
            return {
                "memory": self.memory_entries,
                "user": self.user_entries,
                "memory_usage": f"{self._char_count('memory')}/{self._char_limit('memory')}",
                "user_usage": f"{self._char_count('user')}/{self._char_limit('user')}",
            }
        elif target == "user":
            return {
                "entries": self.user_entries,
                "usage": f"{self._char_count('user')}/{self._char_limit('user')}",
            }
        else:
            return {
                "entries": self.memory_entries,
                "usage": f"{self._char_count('memory')}/{self._char_limit('memory')}",
            }

    def get_system_prompt_content(self) -> str:
        """Get memory content for system prompt injection."""
        parts = []
        if self.memory_entries:
            parts.append(self._render_block("memory", self.memory_entries))
        if self.user_entries:
            parts.append(self._render_block("user", self.user_entries))
        return "\n\n".join(parts)


# Global memory store instance
_memory_store: MemoryStore | None = None


def get_memory_store() -> MemoryStore:
    """Get the global memory store instance."""
    global _memory_store
    if _memory_store is None:
        _memory_store = MemoryStore()
        _memory_store.load_from_disk()
    return _memory_store


def _memory_tool(action: str, target: str = "memory", content: str = "", old_text: str = "") -> str:
    """Memory tool handler.

    Actions:
    - add: Add new entry
    - replace: Replace entry
    - remove: Remove entry
    - read: Read entries
    """
    store = get_memory_store()

    if action == "read":
        result = store.read(target)
    elif action == "add":
        if not content:
            return json.dumps({"success": False, "error": "content required for add"})
        result = store.add(target, content)
    elif action == "replace":
        if not old_text or not content:
            return json.dumps({"success": False, "error": "old_text and content required for replace"})
        result = store.replace(target, old_text, content)
    elif action == "remove":
        if not content:
            return json.dumps({"success": False, "error": "content required for remove"})
        result = store.remove(target, content)
    else:
        result = {"success": False, "error": f"Unknown action: {action}"}

    return json.dumps(result)


def register_tools() -> None:
    """Register memory tools."""
    registry = get_registry()

    registry.register(
        name="memory",
        description="Persistent curated memory. Actions: read, add, replace, remove. Targets: memory, user.",
        schema={
            "type": "object",
            "properties": {
                "action": {
                    "type": "string",
                    "enum": ["read", "add", "replace", "remove"],
                    "description": "Action to perform",
                },
                "target": {
                    "type": "string",
                    "enum": ["memory", "user", "all"],
                    "description": "Memory target",
                    "default": "memory",
                },
                "content": {
                    "type": "string",
                    "description": "Content for add/replace actions",
                },
                "old_text": {
                    "type": "string",
                    "description": "Text to replace (for replace action)",
                },
            },
            "required": ["action"],
        },
        handler=_memory_tool,
    )
