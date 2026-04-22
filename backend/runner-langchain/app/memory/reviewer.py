"""Background reviewer for memory optimization.

Based on Go runner's background_reviewer.go design.
Spawns background reviews after main task completion.
"""

import logging
import threading
from dataclasses import dataclass
from typing import Optional

from app.memory.extractor import MemoryExtractor, ExtractedMemory
from app.memory.store import MemoryStore, EntryType, get_memory_store

logger = logging.getLogger(__name__)


@dataclass
class BackgroundReviewConfig:
    """Configuration for background reviewer."""
    enabled: bool = True
    max_memory: int = 10
    api_key: str = ""
    api_base: str = "https://api.openai.com/v1"
    model: str = "gpt-4o-mini"


# Trigger keywords for memory review
_TRIGGER_KEYWORDS = [
    "记住", "记住这个", "以后要", "下次记得",
    "save", "remember", "keep in mind",
    "don't forget", "never", "always",
]


class BackgroundReviewer:
    """Background memory/skills reviewer.

    Runs in _spawn_background_review mode:
    Executes after main response is sent, not competing for model attention.
    """

    def __init__(
        self,
        config: Optional[BackgroundReviewConfig] = None,
        memory_store: Optional[MemoryStore] = None,
    ):
        self.config = config or BackgroundReviewConfig()
        self.memory_store = memory_store or get_memory_store()
        self.extractor: Optional[MemoryExtractor] = None
        self.callback = None
        self.active_review = False
        self._lock = threading.Lock()

    def _get_extractor(self) -> Optional[MemoryExtractor]:
        """Get or create the memory extractor."""
        if self.extractor is None and self.config.api_key:
            self.extractor = MemoryExtractor(
                api_key=self.config.api_key,
                api_base=self.config.api_base,
                model=self.config.model,
            )
        return self.extractor

    def set_callback(self, callback) -> None:
        """Set callback for review completion."""
        with self._lock:
            self.callback = callback

    def should_review(self) -> bool:
        """Check if review should run."""
        with self._lock:
            return self.config.enabled and not self.active_review

    def _should_trigger_review(self, user_input: str, assistant_output: str) -> bool:
        """Check if conversation should trigger a review.

        Only triggers when conversation contains worth-saving content.
        """
        # If output is very short, probably not worth reviewing
        if len(assistant_output) < 100:
            return False

        combined = (user_input + assistant_output).lower()

        # Check for trigger keywords
        for keyword in _TRIGGER_KEYWORDS:
            if keyword.lower() in combined:
                return True

        # If output is very long or contains important decisions, trigger
        if len(assistant_output) > 2000:
            return True

        return False

    def review_if_needed(
        self,
        user_input: str,
        assistant_output: str,
        session_id: str = "",
        user_id: str = "",
    ) -> None:
        """Check and trigger background review if needed.

        Should be called after main task completes.
        """
        if not self.should_review():
            return

        if not self._should_trigger_review(user_input, assistant_output):
            return

        self._spawn_background_review(user_input, assistant_output, session_id, user_id)

    def _spawn_background_review(
        self,
        user_input: str,
        assistant_output: str,
        session_id: str,
        user_id: str,
    ) -> None:
        """Spawn background review in separate thread."""
        with self._lock:
            if self.active_review:
                return
            self.active_review = True
            callback = self.callback

        logger.info("[BackgroundReviewer] Starting background review")

        def _worker():
            try:
                summary = self._do_memory_review(user_input, assistant_output, session_id, user_id)
                if summary and callback:
                    logger.info("[BackgroundReviewer] Review completed: %s", summary)
                    callback(summary)
            finally:
                with self._lock:
                    self.active_review = False

        thread = threading.Thread(target=_worker, daemon=True)
        thread.start()

    def _do_memory_review(
        self,
        user_input: str,
        assistant_output: str,
        session_id: str,
        user_id: str,
    ) -> str:
        """Perform memory review and save to store."""
        if not self.config.enabled:
            return ""

        extractor = self._get_extractor()
        if not extractor:
            logger.warning("[BackgroundReviewer] No API key configured, skipping review")
            return ""

        try:
            # Extract memories using LLM
            memories = extractor.extract_memories(user_input, assistant_output)

            if not memories:
                logger.info("[BackgroundReviewer] No memories extracted")
                return ""

            logger.info(
                "[BackgroundReviewer] Extracted %d memories: %s",
                len(memories),
                format_memories_for_log(memories),
            )

            # Save memories to store
            saved_count = 0
            for memory in memories[:self.config.max_memory]:
                entry_id = user_id or session_id
                if not entry_id:
                    entry_id = "default"

                # Determine entry type based on memory type
                if memory.memory_type == "user":
                    entry_type = EntryType.USER
                elif memory.memory_type == "project":
                    entry_type = EntryType.AGENT
                else:
                    entry_type = EntryType.SESSION

                # Save to store
                result = self.memory_store.add(
                    entry_type=entry_type,
                    entry_id=entry_id,
                    key=memory.name,
                    content=memory.content,
                )

                if result.get("success"):
                    saved_count += 1

            summary = f"Saved {saved_count}/{len(memories)} memories"
            logger.info("[BackgroundReviewer] %s", summary)
            return summary

        except Exception as e:
            logger.warning("[BackgroundReviewer] Review failed: %s", e)
            return ""

    def get_active_review_status(self) -> bool:
        """Get current review status."""
        with self._lock:
            return self.active_review


def format_memories_for_log(memories: list[ExtractedMemory]) -> str:
    """Format memories for logging."""
    if not memories:
        return ""
    lines = []
    for m in memories:
        lines.append(f"  - [{m.memory_type}] {m.name}: {m.description}")
    return "\n".join(lines)