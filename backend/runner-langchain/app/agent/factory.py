"""Agent factory using langchain."""

from langchain_openai import ChatOpenAI
from langchain_anthropic import ChatAnthropic
from langchain_core.messages import HumanMessage, SystemMessage, AIMessage
from langchain_core.tools import BaseTool
from typing import Optional

from app.api.schemas import ModelConfig


def create_llm(config: ModelConfig) -> ChatOpenAI | ChatAnthropic:
    """Create LLM instance based on model config.

    Args:
        config: Model configuration

    Returns:
        LLM instance

    Raises:
        ValueError: If provider is not supported
    """
    if config.provider == "openai":
        return ChatOpenAI(
            model=config.name,
            api_key=config.api_key,
            base_url=config.api_base,
            temperature=config.temperature,
            max_tokens=config.max_tokens,
        )
    elif config.provider == "anthropic":
        return ChatAnthropic(
            model=config.name,
            api_key=config.api_key,
            temperature=config.temperature,
            max_tokens=config.max_tokens,
        )
    else:
        raise ValueError(f"Unsupported provider: {config.provider}")


def create_react_agent_prompt(
    system_prompt: str,
    tools: list[BaseTool],
) -> str:
    """Create ReAct agent prompt template.

    Args:
        system_prompt: System instruction
        tools: List of available tools

    Returns:
        Formatted prompt string
    """
    tool_descriptions = "\n".join(
        f"- {t.name}: {t.description}" for t in tools
    )

    return f"""You are a helpful AI agent.

## System Prompt
{system_prompt}

## Available Tools
{tool_descriptions}

## Instructions
You can call one or more functions to assist with the user query.

Think silently about what actions to take.
Always use tools to complete tasks.
When using tools, provide concise and relevant responses.
"""


class AgentResult:
    """Result from agent execution."""

    def __init__(
        self,
        output: str,
        success: bool = True,
        error: Optional[str] = None,
        metadata: Optional[dict] = None,
    ):
        self.output = output
        self.success = success
        self.error = error
        self.metadata = metadata or {}

    def __repr__(self) -> str:
        if self.success:
            return f"<AgentResult success=True output={self.output[:100]!r}...>"
        return f"<AgentResult success=False error={self.error!r}>"
