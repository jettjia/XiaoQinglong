"""Memory store and extractor.

Based on Go runner's memory package design.

Modules:
- store: MemoryStore with session/user/agent entries and frozen snapshots
- extractor: MemoryExtractor for LLM-based memory extraction
- reviewer: BackgroundReviewer for async memory optimization
"""

from app.memory.store import (
    MemoryStore,
    MemoryEntry,
    EntryType,
    get_memory_store,
)
from app.memory.extractor import (
    MemoryExtractor,
    ExtractedMemory,
    format_memories_for_log,
)
from app.memory.reviewer import (
    BackgroundReviewer,
    BackgroundReviewConfig,
)
from app.memory.context_compressor import (
    ContextCompressor,
    CompressionResult,
    ConversationMessage,
)

__all__ = [
    "MemoryStore",
    "MemoryEntry",
    "EntryType",
    "get_memory_store",
    "MemoryExtractor",
    "ExtractedMemory",
    "format_memories_for_log",
    "BackgroundReviewer",
    "BackgroundReviewConfig",
    "ContextCompressor",
    "CompressionResult",
    "ConversationMessage",
]