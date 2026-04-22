"""Memory store for session, user, and agent memory.

Based on Go runner's memstore.go design.
Provides:
- Session/User/Agent memory entries
- Frozen snapshots for system prompt stability
- Security scanning for injection patterns
- Disk persistence
"""

import fcntl
import json
import logging
import os
import re
import threading
import time
from dataclasses import dataclass, asdict
from enum import Enum
from pathlib import Path
from typing import Optional

from app.utils.dir import get_memory_dir, get_session_memory_dir, get_user_memory_dir, get_agent_memory_dir

logger = logging.getLogger(__name__)


class EntryType(str, Enum):
    SESSION = "session"
    USER = "user"
    AGENT = "agent"


@dataclass
class MemoryEntry:
    """A single memory entry."""
    type: EntryType
    id: str  # session_id / user_id / agent_id
    key: str
    content: str
    created_at: float
    updated_at: float


# Threat patterns for content scanning
_INJECTION_PATTERNS = [
    re.compile(r"(?i)ignore\s+(previous|all)\s+instructions"),
    re.compile(r"(?i)you\s+are\s+now\s+(a|an)"),
    re.compile(r"(?i)disregard\s+(previous|all)\s+(instructions?|rules?)"),
    re.compile(r"\$[A-Z_]+\s*=.*curl|wget"),
    re.compile(r"authorized_keys"),
    re.compile(r"(?i)\.ssh/"),
]


def _scan_content(content: str) -> Optional[str]:
    """Scan content for injection/exfiltration patterns."""
    for pattern in _INJECTION_PATTERNS:
        if pattern.search(content):
            return f"Blocked: pattern '{pattern.pattern}'"
    return None


class MemoryStore:
    """Thread-safe memory store with frozen snapshots.

    Provides session/user/agent memory with:
    - In-memory entries for fast access
    - Frozen snapshots for system prompt stability
    - Disk persistence
    - Security scanning
    """

    _instance: Optional["MemoryStore"] = None
    _lock = threading.Lock()

    def __init__(self):
        self.base_dir = get_memory_dir()
        self.base_dir.mkdir(parents=True, exist_ok=True)

        # Live state
        self.session_entries: dict[str, list[MemoryEntry]] = {}
        self.user_entries: dict[str, list[MemoryEntry]] = {}
        self.agent_entries: dict[str, list[MemoryEntry]] = {}

        # Frozen snapshots (captured at session start for system prompt)
        self.session_snapshots: dict[str, str] = {}
        self.user_snapshots: dict[str, str] = {}
        self.agent_snapshots: dict[str, str] = {}

        self._lock = threading.Lock()

    @classmethod
    def get_instance(cls) -> "MemoryStore":
        """Get singleton instance."""
        if cls._instance is None:
            with cls._lock:
                if cls._instance is None:
                    cls._instance = cls()
        return cls._instance

    def _entry_file_path(self, entry_type: EntryType, entry_id: str) -> Path:
        """Get the file path for an entry type."""
        if entry_type == EntryType.SESSION:
            base = get_session_memory_dir(entry_id)
        elif entry_type == EntryType.USER:
            base = get_user_memory_dir(entry_id)
        else:
            base = get_agent_memory_dir(entry_id)
        return base / "entries.json"

    def _snapshot_file_path(self, entry_type: EntryType, entry_id: str) -> Path:
        """Get the snapshot file path."""
        if entry_type == EntryType.SESSION:
            base = get_session_memory_dir(entry_id)
        elif entry_type == EntryType.USER:
            base = get_user_memory_dir(entry_id)
        else:
            base = get_agent_memory_dir(entry_id)
        return base / "snapshot.txt"

    def _load_entries(self, entry_type: EntryType, entry_id: str) -> list[MemoryEntry]:
        """Load entries from disk."""
        filepath = self._entry_file_path(entry_type, entry_id)
        if not filepath.exists():
            return []

        try:
            with open(filepath, "r") as f:
                data = json.load(f)
                return [MemoryEntry(**e) for e in data]
        except (json.JSONDecodeError, KeyError):
            return []

    def _save_entries(self, entry_type: EntryType, entry_id: str, entries: list[MemoryEntry]) -> None:
        """Save entries to disk."""
        filepath = self._entry_file_path(entry_type, entry_id)
        filepath.parent.mkdir(parents=True, exist_ok=True)

        with open(filepath, "w") as f:
            json.dump([asdict(e) for e in entries], f)

    def _load_snapshot(self, entry_type: EntryType, entry_id: str) -> Optional[str]:
        """Load frozen snapshot from disk."""
        filepath = self._snapshot_file_path(entry_type, entry_id)
        if not filepath.exists():
            return None

        try:
            return filepath.read_text()
        except OSError:
            return None

    def _save_snapshot(self, entry_type: EntryType, entry_id: str, snapshot: str) -> None:
        """Save frozen snapshot to disk."""
        filepath = self._snapshot_file_path(entry_type, entry_id)
        filepath.parent.mkdir(parents=True, exist_ok=True)
        filepath.write_text(snapshot)

    def add(
        self,
        entry_type: EntryType,
        entry_id: str,
        key: str,
        content: str,
    ) -> dict:
        """Add a memory entry.

        Args:
            entry_type: SESSION, USER, or AGENT
            entry_id: The session/user/agent ID
            key: Memory key
            content: Memory content

        Returns:
            Result dict with success/error
        """
        # Security scan
        scan_error = _scan_content(content)
        if scan_error:
            return {"success": False, "error": scan_error}

        with self._lock:
            # Get the appropriate entries dict
            if entry_type == EntryType.SESSION:
                entries_dict = self.session_entries
            elif entry_type == EntryType.USER:
                entries_dict = self.user_entries
            else:
                entries_dict = self.agent_entries

            # Load from disk if not in memory
            if entry_id not in entries_dict:
                entries_dict[entry_id] = self._load_entries(entry_type, entry_id)

            now = time.time()
            entry = MemoryEntry(
                type=entry_type,
                id=entry_id,
                key=key,
                content=content,
                created_at=now,
                updated_at=now,
            )

            entries_dict[entry_id].append(entry)
            self._save_entries(entry_type, entry_id, entries_dict[entry_id])

            return {"success": True, "message": "Entry added"}

    def get(
        self,
        entry_type: EntryType,
        entry_id: str,
        key: Optional[str] = None,
    ) -> dict:
        """Get memory entries.

        Args:
            entry_type: SESSION, USER, or AGENT
            entry_id: The session/user/agent ID
            key: Optional key filter

        Returns:
            Dict with entries list
        """
        with self._lock:
            if entry_type == EntryType.SESSION:
                entries_dict = self.session_entries
            elif entry_type == EntryType.USER:
                entries_dict = self.user_entries
            else:
                entries_dict = self.agent_entries

            if entry_id not in entries_dict:
                entries_dict[entry_id] = self._load_entries(entry_type, entry_id)

            entries = entries_dict[entry_id]

            if key:
                entries = [e for e in entries if e.key == key]

            return {
                "entries": [
                    {
                        "key": e.key,
                        "content": e.content,
                        "created_at": e.created_at,
                        "updated_at": e.updated_at,
                    }
                    for e in entries
                ],
                "count": len(entries),
            }

    def update(
        self,
        entry_type: EntryType,
        entry_id: str,
        key: str,
        content: str,
    ) -> dict:
        """Update a memory entry by key.

        Args:
            entry_type: SESSION, USER, or AGENT
            entry_id: The session/user/agent ID
            key: Memory key to update
            content: New content

        Returns:
            Result dict
        """
        scan_error = _scan_content(content)
        if scan_error:
            return {"success": False, "error": scan_error}

        with self._lock:
            if entry_type == EntryType.SESSION:
                entries_dict = self.session_entries
            elif entry_type == EntryType.USER:
                entries_dict = self.user_entries
            else:
                entries_dict = self.agent_entries

            if entry_id not in entries_dict:
                entries_dict[entry_id] = self._load_entries(entry_type, entry_id)

            entries = entries_dict[entry_id]
            for entry in entries:
                if entry.key == key:
                    entry.content = content
                    entry.updated_at = time.time()
                    self._save_entries(entry_type, entry_id, entries)
                    return {"success": True, "message": "Entry updated"}

            return {"success": False, "error": f"Key not found: {key}"}

    def delete(
        self,
        entry_type: EntryType,
        entry_id: str,
        key: str,
    ) -> dict:
        """Delete a memory entry by key.

        Args:
            entry_type: SESSION, USER, or AGENT
            entry_id: The session/user/agent ID
            key: Memory key to delete

        Returns:
            Result dict
        """
        with self._lock:
            if entry_type == EntryType.SESSION:
                entries_dict = self.session_entries
            elif entry_type == EntryType.USER:
                entries_dict = self.user_entries
            else:
                entries_dict = self.agent_entries

            if entry_id not in entries_dict:
                return {"success": False, "error": "Entry not found"}

            entries = entries_dict[entry_id]
            new_entries = [e for e in entries if e.key != key]

            if len(new_entries) == len(entries):
                return {"success": False, "error": f"Key not found: {key}"}

            entries_dict[entry_id] = new_entries
            self._save_entries(entry_type, entry_id, new_entries)

            return {"success": True, "message": "Entry deleted"}

    def freeze_snapshot(self, entry_type: EntryType, entry_id: str) -> str:
        """Create a frozen snapshot for system prompt stability.

        This captures the current memory state. Even if memory is modified
        during the session, the snapshot remains stable for the system prompt.

        Args:
            entry_type: SESSION, USER, or AGENT
            entry_id: The session/user/agent ID

        Returns:
            The snapshot content
        """
        with self._lock:
            entries = self.get(entry_type, entry_id)
            content = entries.get("entries", [])

            if not content:
                snapshot = f"[{entry_type.value.upper()} MEMORY: empty]"
            else:
                lines = [f"[{entry_type.value.upper()} MEMORY — {len(content)} entries]"]
                for e in content:
                    lines.append(f"## {e['key']}")
                    lines.append(e["content"])
                    lines.append("")
                snapshot = "\n".join(lines)

            # Store snapshot
            if entry_type == EntryType.SESSION:
                self.session_snapshots[entry_id] = snapshot
            elif entry_type == EntryType.USER:
                self.user_snapshots[entry_id] = snapshot
            else:
                self.agent_snapshots[entry_id] = snapshot

            self._save_snapshot(entry_type, entry_id, snapshot)
            return snapshot

    def get_snapshot(self, entry_type: EntryType, entry_id: str) -> Optional[str]:
        """Get the frozen snapshot (from memory or disk).

        Args:
            entry_type: SESSION, USER, or AGENT
            entry_id: The session/user/agent ID

        Returns:
            The snapshot content or None
        """
        with self._lock:
            # Check memory first
            if entry_type == EntryType.SESSION:
                snapshot = self.session_snapshots.get(entry_id)
            elif entry_type == EntryType.USER:
                snapshot = self.user_snapshots.get(entry_id)
            else:
                snapshot = self.agent_snapshots.get(entry_id)

            if snapshot:
                return snapshot

            # Load from disk
            return self._load_snapshot(entry_type, entry_id)

    def get_all_snapshots(self, session_id: str, user_id: str, agent_id: str) -> str:
        """Get all snapshots combined for system prompt.

        Args:
            session_id: Session ID
            user_id: User ID
            agent_id: Agent ID

        Returns:
            Combined snapshot string
        """
        parts = []

        session_snapshot = self.get_snapshot(EntryType.SESSION, session_id)
        if session_snapshot:
            parts.append(session_snapshot)

        user_snapshot = self.get_snapshot(EntryType.USER, user_id)
        if user_snapshot:
            parts.append(user_snapshot)

        agent_snapshot = self.get_snapshot(EntryType.AGENT, agent_id)
        if agent_snapshot:
            parts.append(agent_snapshot)

        return "\n\n".join(parts) if parts else ""


# Global instance
_memory_store: Optional[MemoryStore] = None


def get_memory_store() -> MemoryStore:
    """Get the global memory store instance."""
    global _memory_store
    if _memory_store is None:
        _memory_store = MemoryStore.get_instance()
    return _memory_store