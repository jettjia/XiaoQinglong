"""Context compression for long conversations.

Based on hermes-agent's context_compressor.py design.
Automatically compacts conversations when they exceed token budget.
"""

import json
import logging
import time
from dataclasses import dataclass
from typing import Optional

logger = logging.getLogger(__name__)

# Token budgets
COMPRESSION_THRESHOLD_TOKENS = 25000
TARGET_COMPRESSED_TOKENS = 15000

# Structured summary template sections
SUMMARY_TEMPLATE = """## Goal
{goal}

## Constraints & Preferences
{constraints}

## Progress
{progress}

## Key Decisions
{decisions}

## Relevant Files
{files}

## Next Steps
{next_steps}

## Critical Context
{critical_context}

## Tools & Patterns
{tools_patterns}
"""


@dataclass
class CompressionResult:
    """Result of context compression."""
    original_count: int
    compressed_count: int
    summary: str
    pruned_count: int
    compressed_at: float


@dataclass
class ConversationMessage:
    """A conversation message."""
    role: str
    content: str
    tool_calls: Optional[list] = None
    tool_results: Optional[list] = None


class ContextCompressor:
    """Compresses long conversations using structured summarization.

    Algorithm (from hermes-agent):
    1. should_compress() - check if conversation exceeds threshold
    2. _prune_old_tool_results() - cheap pre-pass, remove orphaned results
    3. _generate_summary() - LLM-based structured summary of middle messages
    4. _sanitize_tool_pairs() - cleanup mismatched tool calls/results
    5. compress() - main entry point
    """

    def __init__(
        self,
        api_key: str,
        api_base: str = "https://api.openai.com/v1",
        model: str = "gpt-4o-mini",
        threshold_tokens: int = COMPRESSION_THRESHOLD_TOKENS,
        target_tokens: int = TARGET_COMPRESSED_TOKENS,
    ):
        self.api_key = api_key
        self.api_base = api_base.rstrip("/")
        self.model = model
        self.threshold_tokens = threshold_tokens
        self.target_tokens = target_tokens

    def _estimate_tokens(self, text: str) -> int:
        """Estimate token count (rough approximation: 4 chars per token)."""
        return len(text) // 4

    def should_compress(self, messages: list[ConversationMessage]) -> bool:
        """Check if conversation exceeds token threshold."""
        total = 0
        for msg in messages:
            total += self._estimate_tokens(msg.content)
            if msg.tool_results:
                for tr in msg.tool_results:
                    total += self._estimate_tokens(str(tr))
        return total > self.threshold_tokens

    def _prune_old_tool_results(self, messages: list[ConversationMessage]) -> int:
        """Remove tool results for which tool calls were already processed.

        This is a cheap pre-pass before LLM summarization.
        Returns count of pruned entries.
        """
        pruned = 0
        seen_tools = set()

        for msg in messages:
            if msg.tool_calls:
                for tc in msg.tool_calls:
                    if hasattr(tc, 'name') or (isinstance(tc, dict) and 'name' in tc):
                        name = tc.get('name') if isinstance(tc, dict) else tc.name
                        seen_tools.add(name)

            if msg.tool_results and msg.role == "tool":
                # Check if this tool result is for a tool we haven't seen
                # or for a tool that's already been pruned
                tool_name = getattr(msg, 'name', None) or 'unknown'
                if tool_name not in seen_tools:
                    msg.content = "[Tool result pruned]"
                    msg.tool_results = []
                    pruned += 1

        return pruned

    def _sanitize_tool_pairs(self, messages: list[ConversationMessage]) -> None:
        """Remove orphaned tool results without matching tool calls."""
        tool_call_ids = set()
        for msg in messages:
            if msg.tool_calls:
                for tc in msg.tool_calls:
                    tc_id = tc.get('id') if isinstance(tc, dict) else getattr(tc, 'id', None)
                    if tc_id:
                        tool_call_ids.add(tc_id)

        for msg in messages:
            if msg.role == "tool" and msg.tool_results:
                for tr in msg.tool_results:
                    tr_id = tr.get('id') if isinstance(tr, dict) else getattr(tr, 'id', None)
                    if tr_id and tr_id not in tool_call_ids:
                        tr['content'] = "[Tool result sanitized]"

    def _build_summarization_prompt(
        self,
        head: list[ConversationMessage],
        tail: list[ConversationMessage],
    ) -> str:
        """Build prompt for LLM to summarize middle messages."""
        head_text = self._format_messages(head, "HEAD (preserved)")
        tail_text = self._format_messages(tail, "TAIL (preserved)")

        return f"""You are a context compression expert. Summarize the MIDDLE messages into a structured format.

## Instructions
The HEAD and TAIL messages are preserved verbatim. Your job is to summarize the MIDDLE portion (which is too long to keep verbatim).

## HEAD (preserved):
{head_text}

## TAIL (preserved):
{tail_text}

## Your Task
Create a structured summary of the MIDDLE conversation that captures:
1. **Goal**: What was the user trying to accomplish?
2. **Constraints & Preferences**: Any explicit user preferences or constraints mentioned?
3. **Progress**: What was accomplished so far?
4. **Key Decisions**: Important choices or conclusions made
5. **Relevant Files**: Files created or modified
6. **Next Steps**: What should continue from here?
7. **Critical Context**: Information that must not be lost
8. **Tools & Patterns**: Notable tool usage patterns

Be concise but comprehensive. Preserve all critical information.

## Format
Use this template exactly:
{SUMMARY_TEMPLATE}

Fill in each section with the relevant information. If a section has no information, write "N/A".
"""

    def _format_messages(self, messages: list[ConversationMessage], label: str) -> str:
        """Format messages for the prompt."""
        if not messages:
            return f"{label}:\n[No messages]"

        lines = [f"{label}:"]
        for i, msg in enumerate(messages):
            role = msg.role
            content = msg.content[:500] + "..." if len(msg.content) > 500 else msg.content
            lines.append(f"  [{i}] {role}: {content}")
            if msg.tool_calls:
                lines.append(f"       tool_calls: {len(msg.tool_calls)}")
            if msg.tool_results:
                lines.append(f"       tool_results: {len(msg.tool_results)}")
        return "\n".join(lines)

    def _call_llm(self, prompt: str) -> str:
        """Call the LLM API."""
        import httpx

        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {self.api_key}",
        }

        payload = {
            "model": self.model,
            "messages": [{"role": "user", "content": prompt}],
            "temperature": 0.3,
        }

        with httpx.Client(timeout=60.0) as client:
            response = client.post(
                f"{self.api_base}/chat/completions",
                headers=headers,
                json=payload,
            )

        if response.status_code != 200:
            raise RuntimeError(f"LLM returned status {response.status_code}")

        result = response.json()
        return result["choices"][0]["message"]["content"]

    def _generate_summary(
        self,
        head: list[ConversationMessage],
        tail: list[ConversationMessage],
    ) -> str:
        """Generate structured summary using LLM."""
        prompt = self._build_summarization_prompt(head, tail)

        try:
            summary = self._call_llm(prompt)
            logger.info("[ContextCompressor] Generated summary (%d chars)", len(summary))
            return summary
        except Exception as e:
            logger.warning("Failed to generate summary: %s", e)
            return "## Summary\n[Compression failed - conversation truncated]"

    def compress(self, messages: list[ConversationMessage]) -> CompressionResult:
        """Compress conversation using structured summarization.

        Args:
            messages: Full conversation history

        Returns:
            CompressionResult with summary and metadata
        """
        original_count = len(messages)
        start_time = time.time()

        # Step 1: Cheap pre-pass - prune orphaned tool results
        pruned = self._prune_old_tool_results(messages)

        # Step 2: Sanitize tool pairs
        self._sanitize_tool_pairs(messages)

        # Step 3: Calculate sizes
        total_tokens = sum(self._estimate_tokens(m.content) for m in messages)

        if total_tokens <= self.threshold_tokens:
            return CompressionResult(
                original_count=original_count,
                compressed_count=original_count,
                summary="",
                pruned_count=pruned,
                compressed_at=time.time() - start_time,
            )

        # Step 4: Find head (system + first exchange) and tail (recent)
        # Keep head intact, find tail, summarize middle
        head_size = 0
        head_end = 0
        for i, msg in enumerate(messages):
            head_size += self._estimate_tokens(msg.content)
            if head_size > 2000:  # Keep ~2K tokens for head
                break
            head_end = i + 1

        # Tail: keep last ~5K tokens
        tail_start = len(messages)
        tail_size = 0
        for i in range(len(messages) - 1, head_end - 1, -1):
            tail_size += self._estimate_tokens(messages[i].content)
            if tail_size > 5000:
                break
            tail_start = i

        head = messages[:head_end]
        middle = messages[head_end:tail_start]
        tail = messages[tail_start:]

        logger.info(
            "[ContextCompressor] Compressing: head=%d, middle=%d, tail=%d, total=%d tokens",
            sum(self._estimate_tokens(m.content) for m in head),
            sum(self._estimate_tokens(m.content) for m in middle),
            sum(self._estimate_tokens(m.content) for m in tail),
            total_tokens,
        )

        # Step 5: Generate summary of middle
        if middle:
            summary = self._generate_summary(head, tail)
        else:
            summary = ""

        compressed_count = head_end + 1 + (len(messages) - tail_start)

        return CompressionResult(
            original_count=original_count,
            compressed_count=compressed_count,
            summary=summary,
            pruned_count=pruned,
            compressed_at=time.time() - start_time,
        )

    def compress_to_messages(
        self,
        messages: list[ConversationMessage],
    ) -> list[ConversationMessage]:
        """Compress conversation and return new message list.

        Returns messages with:
        - Original head messages preserved
        - Middle replaced with summary message
        - Original tail preserved
        """
        result = self.compress(messages)

        if not result.summary:
            return messages

        # Build new message list
        total_tokens = sum(self._estimate_tokens(m.content) for m in messages)
        head_end = 0
        head_size = 0
        for i, msg in enumerate(messages):
            head_size += self._estimate_tokens(msg.content)
            if head_size > 2000:
                break
            head_end = i + 1

        tail_start = len(messages)
        tail_size = 0
        for i in range(len(messages) - 1, head_end - 1, -1):
            tail_size += self._estimate_tokens(messages[i].content)
            if tail_size > 5000:
                break
            tail_start = i

        new_messages = messages[:head_end].copy()

        # Add summary as a special assistant message
        new_messages.append(ConversationMessage(
            role="assistant",
            content=f"[CONTEXT COMPRESSED - Summary of {head_end}:{tail_start} messages]\n\n{result.summary}",
        ))

        new_messages.extend(messages[tail_start:])

        logger.info(
            "[ContextCompressor] Compressed %d messages to %d",
            result.original_count,
            len(new_messages),
        )

        return new_messages