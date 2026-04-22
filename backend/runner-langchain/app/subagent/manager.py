"""Sub-agent manager using langgraph for task delegation."""

import json
import asyncio
from typing import Optional
from dataclasses import dataclass, field

from app.agent.factory import AgentResult, create_llm
from app.subagent.constants import (
    MAX_CONCURRENT_CHILDREN,
    MAX_DELEGATE_DEPTH,
    DEFAULT_MAX_ITERATIONS,
    CHILD_SYSTEM_PROMPT_TEMPLATE,
    DELEGATE_BLOCKED_TOOLS,
)


@dataclass
class SubAgentResult:
    """Result from sub-agent execution."""
    success: bool
    output: str
    error: Optional[str] = None
    summary: Optional[str] = None


class SubAgentConfig:
    """Sub-agent configuration from request."""

    def __init__(self, config: dict):
        self.id: str = config.get("id", "")
        self.name: str = config.get("name", "")
        self.description: str = config.get("description", "")
        self.prompt: str = config.get("prompt", "")
        self.model_config = config.get("model", {})
        self.tools: list[str] = config.get("tools", [])
        self.max_iterations: int = config.get("max_iterations", DEFAULT_MAX_ITERATIONS)
        self.timeout_ms: int = config.get("timeout_ms", 30000)


class SubAgentManager:
    """Manages sub-agent lifecycle and execution.

    This manager:
    - Stores sub-agent configurations
    - Executes sub-agents in isolation (no parent context)
    - Enforces depth limits and tool restrictions
    - Returns summarized results
    """

    _instance: Optional["SubAgentManager"] = None

    def __init__(self, configs: list[dict] | None = None):
        """Initialize the sub-agent manager.

        Args:
            configs: List of sub-agent configuration dicts
        """
        self._agents: dict[str, SubAgentConfig] = {}
        if configs:
            for cfg in configs:
                agent = SubAgentConfig(cfg)
                self._agents[agent.id] = agent

    @classmethod
    def get_instance(cls) -> "SubAgentManager":
        """Get the singleton instance."""
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance

    @classmethod
    def set_instance(cls, manager: "SubAgentManager") -> None:
        """Set the singleton instance (for testing)."""
        cls._instance = manager

    @classmethod
    def reset_instance(cls) -> None:
        """Reset the singleton instance."""
        cls._instance = None

    def register_agent(self, config: dict) -> None:
        """Register a new sub-agent.

        Args:
            config: Sub-agent configuration dict
        """
        agent = SubAgentConfig(config)
        self._agents[agent.id] = agent

    def list_agents(self) -> list[SubAgentConfig]:
        """List all registered agents."""
        return list(self._agents.values())

    def get_agent(self, agent_id: str) -> Optional[SubAgentConfig]:
        """Get agent by ID."""
        return self._agents.get(agent_id)

    def run(
        self,
        agent_id: str | None,
        goal: str,
        context: str | None = None,
        tools: list[str] | None = None,
        depth: int = 0,
    ) -> SubAgentResult:
        """Run a sub-agent with the given task.

        The sub-agent runs in isolation - it only sees the goal and context,
        not the parent agent's conversation history.

        Args:
            agent_id: Agent ID to use (uses first registered if None)
            goal: Task goal/description
            context: Additional context
            tools: Allowed tools (filtered against blocked tools)
            depth: Current delegation depth

        Returns:
            SubAgentResult with output and success status
        """
        if depth >= MAX_DELEGATE_DEPTH:
            return SubAgentResult(
                success=False,
                output="",
                error=f"max delegate depth {MAX_DELEGATE_DEPTH} exceeded",
            )

        agent = self._resolve_agent(agent_id)
        if agent is None:
            return SubAgentResult(
                success=False,
                output="",
                error=f"no sub-agent configured",
            )

        allowed_tools = self._filter_tools(tools or agent.tools)

        return self._execute_agent(agent, goal, context, allowed_tools, depth)

    def _resolve_agent(self, agent_id: str | None) -> SubAgentConfig | None:
        """Resolve agent by ID or return first available."""
        if agent_id and agent_id in self._agents:
            return self._agents[agent_id]

        if self._agents:
            return next(iter(self._agents.values()))

        return None

    def _filter_tools(self, tools: list[str]) -> list[str]:
        """Filter out blocked tools."""
        return [t for t in tools if t not in DELEGATE_BLOCKED_TOOLS]

    def _execute_agent(
        self,
        agent: SubAgentConfig,
        goal: str,
        context: str | None,
        tools: list[str],
        depth: int,
    ) -> SubAgentResult:
        """Execute the agent synchronously.

        In production, this would use langgraph to run the agent.
        For now, we simulate the execution.
        """
        system_prompt = self._build_system_prompt(goal, context, agent.description)

        try:
            llm = create_llm(agent.model_config)

            from langchain_core.messages import HumanMessage, SystemMessage
            messages = [
                SystemMessage(content=system_prompt),
                HumanMessage(content=goal),
            ]

            response = llm.invoke(messages)
            output = response.content if hasattr(response, "content") else str(response)

            return SubAgentResult(
                success=True,
                output=output,
                summary=self._summarize(output),
            )

        except Exception as e:
            return SubAgentResult(
                success=False,
                output="",
                error=str(e),
            )

    def _build_system_prompt(
        self,
        goal: str,
        context: str | None,
        agent_description: str,
    ) -> str:
        """Build system prompt for sub-agent."""
        return CHILD_SYSTEM_PROMPT_TEMPLATE.format(
            goal=goal,
            context=context or "None",
        )

    def _summarize(self, output: str, max_len: int = 500) -> str:
        """Summarize output by truncating at sentence boundaries."""
        if not output:
            return "(no output)"

        if len(output) <= max_len:
            return output

        truncated = output[:max_len]

        for sep in ["。", ".", "!\n", "?\n", "\n"]:
            idx = truncated.rfind(sep)
            if idx > max_len // 2:
                return truncated[:idx + 1] + "\n[内容已截断]"

        return truncated + "...\n[内容已截断]"


class BatchDelegateResult:
    """Result from batch delegation."""

    def __init__(self, results: list[SubAgentResult]):
        self.results = results

    @property
    def success(self) -> bool:
        return all(r.success for r in self.results)

    def to_json(self) -> str:
        return json.dumps({
            "results": [
                {
                    "success": r.success,
                    "output": r.output,
                    "summary": r.summary,
                    "error": r.error,
                }
                for r in self.results
            ]
        })
