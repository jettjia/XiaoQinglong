"""Checkpoint and Resume for agent execution state.

Provides persistent storage of agent execution state so sessions can be
paused, resumed, or recovered after interruption.

Based on hermes-agent's state store design.
"""

import json
import logging
import os
import threading
from dataclasses import dataclass, asdict
from datetime import datetime
from pathlib import Path
from typing import Any, Optional

import sqlite3

logger = logging.getLogger(__name__)

# Checkpoint directory (follows xqldir design)
CHECKPOINT_DIR = Path.home() / ".xiaoqinglong" / "checkpoints"


@dataclass
class Checkpoint:
    """Represents a checkpoint of agent execution state."""
    checkpoint_id: str
    session_id: str
    created_at: str
    updated_at: str
    iteration: int
    messages: list[dict]
    tool_results: dict
    metadata: dict
    finished: bool = False


class CheckpointStore:
    """Stores agent execution checkpoints.

    Uses SQLite for persistent storage with:
    - Session metadata
    - Full message history
    - Tool call records
    - State for resuming
    """

    def __init__(self, db_path: Optional[Path] = None):
        self.db_path = db_path or CHECKPOINT_DIR / "state.db"
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        self._conn: Optional[sqlite3.Connection] = None
        self._lock = threading.Lock()
        self._init_db()

    def _init_db(self) -> None:
        """Initialize the database schema."""
        with self._lock:
            conn = self._get_conn()
            cursor = conn.cursor()

            cursor.execute("""
                CREATE TABLE IF NOT EXISTS checkpoints (
                    checkpoint_id TEXT PRIMARY KEY,
                    session_id TEXT NOT NULL,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    iteration INTEGER DEFAULT 0,
                    messages TEXT NOT NULL,
                    tool_results TEXT,
                    metadata TEXT,
                    finished INTEGER DEFAULT 0
                )
            """)

            cursor.execute("""
                CREATE INDEX IF NOT EXISTS idx_checkpoints_session
                ON checkpoints(session_id)
            """)

            cursor.execute("""
                CREATE TABLE IF NOT EXISTS sessions (
                    session_id TEXT PRIMARY KEY,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    ended_at TEXT,
                    end_reason TEXT,
                    model TEXT,
                    metadata TEXT,
                    finished INTEGER DEFAULT 0
                )
            """)

            conn.commit()

    def _get_conn(self) -> sqlite3.Connection:
        """Get database connection."""
        if self._conn is None:
            self._conn = sqlite3.connect(str(self.db_path))
            self._conn.row_factory = sqlite3.Row
        return self._conn

    def create_session(self, session_id: str, metadata: dict | None = None) -> None:
        """Create a new session record."""
        now = datetime.utcnow().isoformat()
        with self._lock:
            conn = self._get_conn()
            conn.execute(
                """
                INSERT OR REPLACE INTO sessions (session_id, created_at, updated_at, metadata)
                VALUES (?, ?, ?, ?)
                """,
                (session_id, now, now, json.dumps(metadata or {})),
            )
            conn.commit()

    def create_checkpoint(
        self,
        checkpoint_id: str,
        session_id: str,
        iteration: int,
        messages: list[dict],
        tool_results: dict | None = None,
        metadata: dict | None = None,
        finished: bool = False,
    ) -> None:
        """Create or update a checkpoint."""
        now = datetime.utcnow().isoformat()
        with self._lock:
            conn = self._get_conn()
            conn.execute(
                """
                INSERT OR REPLACE INTO checkpoints
                (checkpoint_id, session_id, created_at, updated_at, iteration,
                 messages, tool_results, metadata, finished)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    checkpoint_id,
                    session_id,
                    now,
                    now,
                    iteration,
                    json.dumps(messages),
                    json.dumps(tool_results or {}),
                    json.dumps(metadata or {}),
                    1 if finished else 0,
                ),
            )
            conn.execute(
                """
                UPDATE sessions SET updated_at = ?, finished = ?
                WHERE session_id = ?
                """,
                (now, 1 if finished else 0, session_id),
            )
            conn.commit()

    def get_checkpoint(self, checkpoint_id: str) -> Optional[Checkpoint]:
        """Get a checkpoint by ID."""
        with self._lock:
            conn = self._get_conn()
            cursor = conn.execute(
                "SELECT * FROM checkpoints WHERE checkpoint_id = ?",
                (checkpoint_id,),
            )
            row = cursor.fetchone()

            if not row:
                return None

            return Checkpoint(
                checkpoint_id=row["checkpoint_id"],
                session_id=row["session_id"],
                created_at=row["created_at"],
                updated_at=row["updated_at"],
                iteration=row["iteration"],
                messages=json.loads(row["messages"]),
                tool_results=json.loads(row["tool_results"] or "{}"),
                metadata=json.loads(row["metadata"] or "{}"),
                finished=bool(row["finished"]),
            )

    def get_latest_checkpoint(self, session_id: str) -> Optional[Checkpoint]:
        """Get the latest checkpoint for a session."""
        with self._lock:
            conn = self._get_conn()
            cursor = conn.execute(
                """
                SELECT * FROM checkpoints
                WHERE session_id = ?
                ORDER BY updated_at DESC
                LIMIT 1
                """,
                (session_id,),
            )
            row = cursor.fetchone()

            if not row:
                return None

            return Checkpoint(
                checkpoint_id=row["checkpoint_id"],
                session_id=row["session_id"],
                created_at=row["created_at"],
                updated_at=row["updated_at"],
                iteration=row["iteration"],
                messages=json.loads(row["messages"]),
                tool_results=json.loads(row["tool_results"] or "{}"),
                metadata=json.loads(row["metadata"] or "{}"),
                finished=bool(row["finished"]),
            )

    def list_sessions(self) -> list[dict]:
        """List all sessions."""
        with self._lock:
            conn = self._get_conn()
            cursor = conn.execute(
                """
                SELECT s.*,
                       (SELECT COUNT(*) FROM checkpoints c WHERE c.session_id = s.session_id) as checkpoint_count
                FROM sessions s
                ORDER BY s.updated_at DESC
                LIMIT 50
                """
            )
            rows = cursor.fetchall()
            return [
                {
                    "session_id": row["session_id"],
                    "created_at": row["created_at"],
                    "updated_at": row["updated_at"],
                    "ended_at": row["ended_at"],
                    "end_reason": row["end_reason"],
                    "finished": bool(row["finished"]),
                    "checkpoint_count": row["checkpoint_count"],
                }
                for row in rows
            ]

    def delete_session(self, session_id: str) -> None:
        """Delete a session and its checkpoints."""
        with self._lock:
            conn = self._get_conn()
            conn.execute("DELETE FROM checkpoints WHERE session_id = ?", (session_id,))
            conn.execute("DELETE FROM sessions WHERE session_id = ?", (session_id,))
            conn.commit()

    def close(self) -> None:
        """Close the database connection."""
        with self._lock:
            if self._conn:
                self._conn.close()
                self._conn = None


# Global checkpoint store instance
_checkpoint_store: Optional[CheckpointStore] = None


def get_checkpoint_store() -> CheckpointStore:
    """Get the global checkpoint store instance."""
    global _checkpoint_store
    if _checkpoint_store is None:
        _checkpoint_store = CheckpointStore()
    return _checkpoint_store
