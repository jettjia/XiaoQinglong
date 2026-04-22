"""Parallel execution tool for read-only operations.

Based on Go runner's parallel.go and hermes-agent's concurrent execution design.
Allows multiple read-only tool calls to be executed concurrently.
"""

import json
import logging
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from typing import Any, Optional

from app.tools.registry import get_registry

logger = logging.getLogger(__name__)

# Max concurrent workers
MAX_WORKERS = 8

# Read-only tools that can be parallelized
READ_ONLY_TOOLS = frozenset([
    "glob",
    "grep",
    "file_read",
    "read_file",
    "web_search",
    "web_fetch",
    "skills_list",
    "skill_view",
    "skills_categories",
    "memory",
    "todo_list",
    "task_list",
    "get_state",
])


@dataclass
class ToolCall:
    """A single tool call."""
    name: str
    arguments: dict


@dataclass
class ToolResult:
    """Result of a tool execution."""
    name: str
    success: bool
    result: str
    error: Optional[str] = None


class ParallelToolExecutor:
    """Executes multiple read-only tool calls concurrently.

    Only read-only tools are executed in parallel. Write operations
    are executed sequentially to avoid conflicts.
    """

    def __init__(self, max_workers: int = MAX_WORKERS):
        self.max_workers = max_workers
        self.registry = get_registry()

    def _is_read_only(self, tool_name: str) -> bool:
        """Check if a tool is read-only."""
        tool_info = self.registry.get_tool(tool_name.lower())
        if tool_info:
            return tool_info.get("is_read_only", False)
        return tool_name.lower() in READ_ONLY_TOOLS

    def _execute_single(self, tool_call: ToolCall) -> ToolResult:
        """Execute a single tool call."""
        try:
            tool_info = self.registry.get_tool(tool_call.name.lower())
            if not tool_info:
                return ToolResult(
                    name=tool_call.name,
                    success=False,
                    result="",
                    error=f"Tool not found: {tool_call.name}",
                )

            handler = tool_info.get("handler")
            if not handler:
                return ToolResult(
                    name=tool_call.name,
                    success=False,
                    result="",
                    error=f"Tool has no handler: {tool_call.name}",
                )

            result = handler(**tool_call.arguments)
            return ToolResult(
                name=tool_call.name,
                success=True,
                result=result,
            )

        except Exception as e:
            logger.warning("[ParallelToolExecutor] Tool '%s' failed: %s", tool_call.name, e)
            return ToolResult(
                name=tool_call.name,
                success=False,
                result="",
                error=str(e),
            )

    def execute_parallel(
        self,
        tool_calls: list[dict],
    ) -> list[dict]:
        """Execute multiple tool calls, parallelizing read-only operations.

        Args:
            tool_calls: List of tool calls, each with 'name' and 'arguments' keys

        Returns:
            List of results in the same order as input
        """
        if not tool_calls:
            return []

        parsed_calls = []
        call_index = {}
        readonly_indices = []
        write_indices = []

        for i, tc in enumerate(tool_calls):
            name = tc.get("name", "").lower()
            arguments = tc.get("arguments", {})

            call = ToolCall(name=name, arguments=arguments)
            parsed_calls.append(call)

            if self._is_read_only(name):
                readonly_indices.append(i)
            else:
                write_indices.append(i)

        results: list[Optional[ToolResult]] = [None] * len(tool_calls)

        # Execute read-only tools in parallel
        if readonly_indices:
            with ThreadPoolExecutor(max_workers=min(self.max_workers, len(readonly_indices))) as executor:
                futures = {}
                for idx in readonly_indices:
                    call = parsed_calls[idx]
                    future = executor.submit(self._execute_single, call)
                    futures[future] = idx

                for future in as_completed(futures):
                    idx = futures[future]
                    try:
                        results[idx] = future.result()
                    except Exception as e:
                        results[idx] = ToolResult(
                            name=parsed_calls[idx].name,
                            success=False,
                            result="",
                            error=str(e),
                        )

        # Execute write tools sequentially (in order)
        for idx in write_indices:
            results[idx] = self._execute_single(parsed_calls[idx])

        # Convert to dict format
        return [
            {
                "name": r.name,
                "success": r.success,
                "result": r.result,
                "error": r.error,
            }
            if r else {
                "name": "unknown",
                "success": False,
                "result": "",
                "error": "Execution failed",
            }
            for r in results
        ]


def _parallel(tool_calls: list[dict]) -> str:
    """Execute multiple tool calls in parallel.

    Args:
        tool_calls: List of tool calls. Each call should have:
            - name: tool name
            - arguments: dict of tool arguments

    Returns:
        JSON string with results
    """
    try:
        executor = ParallelToolExecutor()
        results = executor.execute_parallel(tool_calls)
        return json.dumps({
            "success": True,
            "results": results,
            "count": len(results),
        })
    except Exception as e:
        logger.error("[parallel] Execution failed: %s", e)
        return json.dumps({
            "success": False,
            "error": str(e),
        })


def register_tools() -> None:
    """Register parallel tool."""
    registry = get_registry()

    registry.register(
        name="parallel",
        description="Execute multiple read-only tool calls in parallel for efficiency. Write operations are executed sequentially.",
        schema={
            "type": "object",
            "properties": {
                "tool_calls": {
                    "type": "array",
                    "description": "List of tool calls to execute",
                    "items": {
                        "type": "object",
                        "properties": {
                            "name": {
                                "type": "string",
                                "description": "Tool name (e.g., 'glob', 'grep', 'file_read')",
                            },
                            "arguments": {
                                "type": "object",
                                "description": "Tool arguments as key-value pairs",
                            },
                        },
                        "required": ["name"],
                    },
                },
            },
            "required": ["tool_calls"],
        },
        handler=lambda **kwargs: _parallel(kwargs.get("tool_calls", [])),
    )