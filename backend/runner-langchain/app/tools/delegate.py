"""Delegate tool for sub-agent task delegation."""

import json
import asyncio
from typing import TYPE_CHECKING
from langchain_core.tools import tool

if TYPE_CHECKING:
    from app.subagent.manager import SubAgentManager

MAX_CONCURRENT_CHILDREN = 3
MAX_DEPTH = 2

DELEGATE_BLOCKED_TOOLS = frozenset([
    "delegate_to_agent",
    "clarify",
    "memory",
    "send_message",
])


class DelegateTool:
    """Delegate tool that allows agents to spawn sub-agents for parallel task execution.

    This tool supports:
    - Single task delegation
    - Batch parallel delegation (up to 3 concurrent)
    - Depth limiting (max 2 levels: parent -> child -> grandchild)
    - Blocked tools filtering
    """

    def __init__(self, sub_agent_manager: "SubAgentManager"):
        self.manager = sub_agent_manager

    def get_tool(self):
        """Get the delegate_to_agent tool."""
        return delegate_to_agent


@tool
def delegate_to_agent(
    agent_id: str | None = None,
    task: str | None = None,
    tasks: str | None = None,
    context: str | None = None,
    depth: int = 0,
) -> str:
    """Delegate tasks to sub-agents for parallel execution.

    Use this tool when:
    - Multiple independent tasks can be executed in parallel
    - A task requires specialized capabilities
    - You want to offload work to a sub-agent

    Args:
        agent_id: The ID of the sub-agent to delegate to (optional, uses first configured if not specified)
        task: Single task description (legacy mode, use 'tasks' for batch)
        tasks: JSON string array of tasks for batch delegation. Each task is an object
               with: goal (string), context (string, optional), tools (array, optional)
               Example: '[{"goal": "search web", "context": "latest AI news"}, {"goal": "read file"}]'
        context: Additional context for the task
        depth: Current delegation depth (auto-managed, don't set manually)

    Returns:
        JSON string with results array, each containing success (bool) and summary (string)

    Max 3 tasks can be delegated in parallel. Max delegation depth is 2 (parent->child->grandchild).
    """
    if depth >= MAX_DEPTH:
        return json.dumps({
            "error": f"max delegate depth {MAX_DEPTH} exceeded",
            "results": [],
        })

    task_list = []

    if tasks:
        try:
            raw_tasks = json.loads(tasks)
            for t in raw_tasks[:MAX_CONCURRENT_CHILDREN]:
                task_list.append({
                    "goal": t.get("goal", ""),
                    "context": t.get("context", ""),
                    "tools": t.get("tools", []),
                })
        except (json.JSONDecodeError, AttributeError) as e:
            return json.dumps({
                "error": f"invalid tasks format: {e}",
                "results": [],
            })
    elif task:
        task_list.append({
            "goal": task,
            "context": context or "",
            "tools": [],
        })
    else:
        return json.dumps({
            "error": "either task or tasks must be provided",
            "results": [],
        })

    results = []
    for t in task_list:
        result = _execute_single_task(agent_id, t, depth)
        results.append(result)

    return json.dumps({"results": results})


def _execute_single_task(
    agent_id: str | None,
    task: dict,
    depth: int,
) -> dict:
    """Execute a single delegated task.

    Args:
        agent_id: Sub-agent ID
        task: Task dict with goal, context, tools
        depth: Current depth

    Returns:
        Result dict with success and summary
    """
    from app.subagent.manager import SubAgentManager

    goal = task.get("goal", "")
    task_context = task.get("context", "")

    if not goal:
        return {"success": False, "summary": "empty goal"}

    try:
        result = SubAgentManager.get_instance().run(
            agent_id=agent_id,
            goal=goal,
            context=task_context,
            tools=task.get("tools", []),
            depth=depth,
        )

        if result.success:
            return {
                "success": True,
                "summary": _summarize_result(result.output),
            }
        else:
            return {
                "success": False,
                "summary": result.error or "unknown error",
            }
    except Exception as e:
        return {
            "success": False,
            "summary": f"execution error: {str(e)}",
        }


def _summarize_result(output: str, max_len: int = 500) -> str:
    """Summarize result by truncating to max length at sentence boundaries.

    Args:
        output: Full output
        max_len: Maximum length

    Returns:
        Summarized output
    """
    if not output:
        return "(no output)"

    if len(output) <= max_len:
        return output

    truncated = output[:max_len]

    for sep in ["。", ".", "\n"]:
        idx = truncated.rfind(sep)
        if idx > max_len // 2:
            return truncated[:idx + 1] + "\n[内容已截断]"

    return truncated + "...\n[内容已截断]"


def filter_blocked_tools(tools: list[str]) -> list[str]:
    """Filter out blocked tools from the allowed tools list.

    Args:
        tools: List of tool names

    Returns:
        Filtered list without blocked tools
    """
    return [t for t in tools if t not in DELEGATE_BLOCKED_TOOLS]
