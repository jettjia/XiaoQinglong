"""ReAct agent implementation using langgraph.

Based on hermes-agent's agent_loop.py design.
"""

import json
import logging
from typing import Annotated, Sequence, TypedDict
from dataclasses import dataclass

from langchain_core.messages import BaseMessage, HumanMessage, AIMessage, SystemMessage, ToolMessage
from langchain_core.tools import BaseTool
from langgraph.graph import StateGraph, END
from langgraph.prebuilt import ToolNode

from app.agent.factory import create_llm, AgentResult
from app.agent.skills import build_skills_system_prompt
from app.api.schemas import ModelConfig, RunOptions

logger = logging.getLogger(__name__)


class AgentState(TypedDict):
    """State for the ReAct agent."""
    messages: Annotated[Sequence[BaseMessage], "The conversation messages"]
    iteration: int
    tool_results: dict


@dataclass
class ReActResult:
    """Result from ReAct agent execution."""
    messages: list[BaseMessage]
    iterations: int
    finished_naturally: bool
    output: str


class ReActAgent:
    """ReAct agent using langgraph.

    Based on hermes-agent's agent_loop design.
    """

    def __init__(
        self,
        llm,
        tools: list[BaseTool],
        system_prompt: str,
        max_iterations: int = 50,
        max_tool_calls: int = 30,
    ):
        self.llm = llm
        self.tools = tools
        self.system_prompt = system_prompt
        self.max_iterations = max_iterations
        self.max_tool_calls = max_tool_calls

        # Build tool node for langgraph
        self.tool_node = ToolNode(tools)

        # Build the graph
        self.graph = self._build_graph()

    def _build_graph(self) -> StateGraph:
        """Build the ReAct state graph."""
        workflow = StateGraph(AgentState)

        # Add nodes
        workflow.add_node("agent", self._call_model)
        workflow.add_node("tools", self.tool_node)

        # Set entry point
        workflow.set_entry_point("agent")

        # Add edges
        workflow.add_conditional_edges(
            "agent",
            self._should_continue,
            {
                "continue": "tools",
                "end": END,
            }
        )
        workflow.add_edge("tools", "agent")

        return workflow.compile()

    def _should_continue(self, state: AgentState) -> str:
        """Determine if we should continue or end."""
        messages = state["messages"]
        last_message = messages[-1]

        # Check if last message has tool calls
        if hasattr(last_message, "tool_calls") and last_message.tool_calls:
            return "continue"

        return "end"

    def _call_model(self, state: AgentState) -> AgentState:
        """Call the LLM with tools."""
        messages = list(state["messages"])

        # Add system prompt if not present
        if not any(isinstance(m, SystemMessage) for m in messages):
            messages.insert(0, SystemMessage(content=self.system_prompt))

        # Count existing tool calls
        tool_calls_count = sum(
            len(m.tool_calls) if hasattr(m, "tool_calls") and m.tool_calls else 0
            for m in messages
        )

        # Stop if exceeded max tool calls
        if tool_calls_count >= self.max_tool_calls:
            logger.warning("Max tool calls (%d) exceeded", self.max_tool_calls)
            return state

        try:
            response = self.llm.invoke(messages)
            messages.append(response)
        except Exception as e:
            logger.error(f"LLM call failed: {e}")
            error_msg = AIMessage(content=f"Error: {str(e)}")
            messages.append(error_msg)

        return {"messages": messages, "iteration": state.get("iteration", 0) + 1}

    def run(self, initial_messages: list[BaseMessage]) -> ReActResult:
        """Run the agent.

        Args:
            initial_messages: Initial conversation messages

        Returns:
            ReActResult with final state
        """
        initial_state = {
            "messages": initial_messages,
            "iteration": 0,
            "tool_results": {},
        }

        try:
            final_state = self.graph.invoke(initial_state, {"recursion_limit": self.max_iterations})
        except Exception as e:
            logger.error(f"Agent graph execution failed: {e}")
            return ReActResult(
                messages=initial_state["messages"],
                iterations=0,
                finished_naturally=False,
                output=f"Error: {str(e)}",
            )

        messages = final_state["messages"]
        iterations = final_state.get("iteration", 0)

        # Extract final output
        final_output = ""
        if messages:
            last_msg = messages[-1]
            if hasattr(last_msg, "content"):
                final_output = last_msg.content
            elif isinstance(last_msg, AIMessage):
                final_output = last_msg.content or ""

        finished = iterations < self.max_iterations and not final_output.startswith("Error")

        return ReActResult(
            messages=messages,
            iterations=iterations,
            finished_naturally=finished,
            output=final_output,
        )

    async def run_async(self, initial_messages: list[BaseMessage]) -> ReActResult:
        """Run the agent asynchronously."""
        import asyncio

        # Wrap sync run in async
        loop = asyncio.get_event_loop()
        return await loop.run_in_executor(None, self.run, initial_messages)

    def run_streaming(self, initial_messages: list[BaseMessage]):
        """Run the agent with streaming response.

        Yields:
            Chunks of the response as they become available.
        """
        initial_state = {
            "messages": initial_messages,
            "iteration": 0,
            "tool_results": {},
        }

        try:
            for event in self.graph.stream(initial_state, {"recursion_limit": self.max_iterations}):
                # Events are state dicts from each node
                if "agent" in event:
                    state = event["agent"]
                    messages = state.get("messages", [])
                    if messages:
                        last_msg = messages[-1]
                        if hasattr(last_msg, "content") and last_msg.content:
                            yield {"type": "content", "content": last_msg.content}
                        if hasattr(last_msg, "tool_calls") and last_msg.tool_calls:
                            yield {"type": "tool_calls", "tool_calls": last_msg.tool_calls}
                elif "tools" in event:
                    state = event["tools"]
                    messages = state.get("messages", [])
                    for msg in messages:
                        if isinstance(msg, ToolMessage):
                            yield {"type": "tool_result", "tool": msg.name or "unknown", "result": msg.content}
        except Exception as e:
            logger.error(f"Agent streaming failed: %s", e)
            yield {"type": "error", "error": str(e)}

    def run_with_callback(self, initial_messages: list[BaseMessage], callback):
        """Run the agent with progress callback.

        Args:
            initial_messages: Initial conversation messages
            callback: Function called with each content chunk
        """
        for chunk in self.run_streaming(initial_messages):
            if chunk.get("type") == "content":
                callback(chunk["content"])


def create_react_agent(
    model_config: ModelConfig,
    tools: list[BaseTool],
    system_prompt: str,
    options: RunOptions | None = None,
    skills: list[str] | None = None,
    toolsets: list[str] | None = None,
) -> ReActAgent:
    """Create a ReAct agent.

    Args:
        model_config: Model configuration
        tools: List of available tools
        system_prompt: System prompt
        options: Runtime options
        skills: List of skill names to activate
        toolsets: List of toolset names to filter skills

    Returns:
        ReActAgent instance
    """
    options = options or RunOptions()

    llm = create_llm(model_config)

    # Bind tools to LLM if supported
    if hasattr(llm, "bind_tools"):
        llm = llm.bind_tools(tools)

    # Build full system prompt with skills section
    tool_names = [t.name for t in tools] if tools else None
    skills_prompt = build_skills_system_prompt(
        available_tools=tool_names,
        toolsets=toolsets,
    )

    full_system_prompt = system_prompt
    if skills_prompt:
        full_system_prompt = system_prompt + skills_prompt

    return ReActAgent(
        llm=llm,
        tools=tools,
        system_prompt=full_system_prompt,
        max_iterations=options.max_iterations,
        max_tool_calls=options.max_tool_calls,
    )
