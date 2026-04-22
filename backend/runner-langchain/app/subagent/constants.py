"""Constants for sub-agent delegation."""

MAX_CONCURRENT_CHILDREN = 3
MAX_DELEGATE_DEPTH = 2
DEFAULT_MAX_ITERATIONS = 50

DELEGATE_BLOCKED_TOOLS = frozenset([
    "delegate_to_agent",
    "clarify",
    "memory",
    "send_message",
    "execute_code",
])

CHILD_SYSTEM_PROMPT_TEMPLATE = """You are a focused subagent working on a specific delegated task.

YOUR TASK:
{goal}

CONTEXT:
{context}

Complete this task using the tools available to you.
When finished, provide a clear, concise summary of:
- What you did
- What you found or accomplished
- Any files you created or modified
- Any issues encountered
"""
