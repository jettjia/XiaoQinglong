"""Tool implementations."""

from app.tools.registry import ToolRegistry, get_registry
from app.tools.delegate import delegate_to_agent, DelegateTool


def register_all_tools() -> None:
    """Register all built-in tools."""
    from app.tools.delegate import delegate_to_agent
    from app.tools.registry import get_registry

    registry = get_registry()

    # Register delegate tool
    registry.register(
        name="delegate_to_agent",
        description="Delegate tasks to sub-agents for parallel execution",
        schema={
            "type": "object",
            "properties": {
                "agent_id": {"type": "string", "description": "Sub-agent ID"},
                "task": {"type": "string", "description": "Single task description"},
                "tasks": {"type": "string", "description": "JSON array of tasks for batch delegation"},
                "context": {"type": "string", "description": "Additional context"},
                "depth": {"type": "integer", "description": "Current delegation depth", "default": 0},
            },
        },
        handler=delegate_to_agent,
    )

    # Register other tools
    from app.tools import bash, file_ops, web, browser, memory, glob, grep, skills
    from app.tools import edit, task, todo, plan_mode, question
    from app.tools import sleep, parallel, result_limiter, skill_script, html_interpreter
    from app.tools import loop
    bash.register_tools()
    file_ops.register_tools()
    web.register_tools()
    browser.register_tools()
    memory.register_tools()
    glob.register_tools()
    grep.register_tools()
    skills.register_tools()
    edit.register_tools()
    task.register_tools()
    todo.register_tools()
    plan_mode.register_tools()
    question.register_tools()
    sleep.register_tools()
    parallel.register_tools()
    result_limiter.register_tools()
    skill_script.register_tools()
    html_interpreter.register_tools()
    loop.register_tools()


__all__ = [
    "ToolRegistry",
    "get_registry",
    "register_all_tools",
    "delegate_to_agent",
    "DelegateTool",
]
