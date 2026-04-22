"""Loop Controller for continuous agent execution.

Based on Go runner's loop_controller.go design.
Provides continuous execution with intervals and stop conditions.
"""

import asyncio
import json
import logging
import re
import threading
import time
import uuid
from dataclasses import dataclass, asdict, field
from typing import Any, Callable, Optional

from langchain_core.messages import BaseMessage, HumanMessage, SystemMessage, AIMessage

from app.agent.factory import create_llm, AgentResult
from app.agent.react import ReActAgent, create_react_agent
from app.api.schemas import ModelConfig, RunOptions

logger = logging.getLogger(__name__)


@dataclass
class LoopIterationResult:
    """Result of a single loop iteration."""
    iteration: int
    content: str
    finish_reason: str
    tool_calls: list[dict] = field(default_factory=list)
    tokens_used: int = 0
    done: bool = False
    error: Optional[str] = None


@dataclass
class LoopResponse:
    """Response from loop execution."""
    loop_id: str
    status: str  # running, completed, stopped
    iterations: list[LoopIterationResult]
    total_tokens: int
    total_iterations: int
    final_content: str


@dataclass
class LoopOptions:
    """Options for loop execution."""
    max_iterations: int = 100
    interval: str = "5s"  # e.g., "5s", "1m", "1h"
    stop_condition: str = ""  # e.g., "LOOP_COMPLETE"
    checkpoint_id: Optional[str] = None


@dataclass
class LoopRequest:
    """Request to start a loop."""
    prompt: str
    model_config: ModelConfig
    system_prompt: str = ""
    context: dict[str, Any] = field(default_factory=dict)
    options: LoopOptions = field(default_factory=LoopOptions)
    messages: list[dict] = field(default_factory=list)
    tools: list[Any] = field(default_factory=list)


class StreamEvent:
    """Stream event for loop."""
    def __init__(self, event_type: str, data: dict[str, Any]):
        self.type = event_type
        self.data = data


class LoopController:
    """Controls continuous loop execution.

    Based on Go runner's LoopController design.
    Supports:
    - Continuous execution with configurable intervals
    - Stop conditions (e.g., "LOOP_COMPLETE")
    - Message accumulation across iterations
    - Both streaming and non-streaming modes
    """

    def __init__(self, request: LoopRequest):
        self.request = request
        self.loop_id = f"loop-{uuid.uuid4().hex[:12]}"
        self.status = "running"
        self.stop_chan: Optional[threading.Event] = None
        self._lock = threading.Lock()
        self.accumulated_messages: list[dict] = []

    def _get_session_id(self) -> str:
        """Get session ID from context."""
        ctx = self.request.context or {}
        return ctx.get("session_id", "")

    def _parse_interval(self, interval: str) -> float:
        """Parse interval string to seconds.

        Supports: 5s, 1m, 1h, 1d
        """
        interval = interval.strip()
        if not interval:
            return 0.0

        match = re.match(r"^(\d+)(s|m|h|d)?$", interval)
        if not match:
            return 0.0

        value = int(match.group(1))
        unit = match.group(2) or "s"

        multipliers = {
            "s": 1,
            "m": 60,
            "h": 3600,
            "d": 86400,
        }
        return value * multipliers.get(unit, 1)

    def _check_stop_condition(self, result: LoopIterationResult) -> bool:
        """Check if stop condition is met."""
        if not self.request.options.stop_condition:
            return False
        return self.request.options.stop_condition in result.content

    def _build_messages(self) -> list[dict]:
        """Build messages for next iteration."""
        messages = list(self.accumulated_messages)
        messages.append({"role": "user", "content": self.request.prompt})
        return messages

    def _convert_messages(self, messages: list[dict]) -> list[BaseMessage]:
        """Convert dict messages to BaseMessage list."""
        result = []
        for msg in messages:
            role = msg.get("role", "user")
            content = msg.get("content", "")
            if role == "system":
                result.append(SystemMessage(content=content))
            elif role == "user":
                result.append(HumanMessage(content=content))
            elif role == "assistant":
                result.append(AIMessage(content=content))
        return result

    def _append_to_history(self, content: str) -> None:
        """Append exchange to accumulated messages."""
        max_history = 100

        self.accumulated_messages.append({"role": "user", "content": self.request.prompt})
        self.accumulated_messages.append({"role": "assistant", "content": content})

        if len(self.accumulated_messages) > max_history:
            self.accumulated_messages = self.accumulated_messages[-max_history:]

        logger.info("[LoopController] Accumulated %d messages", len(self.accumulated_messages))

    def _run_single_iteration(self, iteration_num: int) -> LoopIterationResult:
        """Run a single iteration of the loop."""
        result = LoopIterationResult(
            iteration=iteration_num,
            content="",
            finish_reason="",
        )

        try:
            messages = self._convert_messages(self._build_messages())

            if self.request.system_prompt:
                if not any(isinstance(m, SystemMessage) for m in messages):
                    messages.insert(0, SystemMessage(content=self.request.system_prompt))

            options = RunOptions(
                max_iterations=30,
                max_tool_calls=20,
            )

            agent = create_react_agent(
                model_config=self.request.model_config,
                tools=self.request.tools,
                system_prompt=self.request.system_prompt,
                options=options,
            )

            agent_result = agent.run(messages)

            result.content = agent_result.output
            result.finish_reason = "stop" if agent_result.finished_naturally else "max_iterations"
            result.tokens_used = 0  # Would need to extract from metadata
            result.done = agent_result.finished_naturally

            self._append_to_history(result.content)

        except Exception as e:
            logger.error("[LoopController] Iteration %d failed: %s", iteration_num, e)
            result.error = str(e)

        return result

    def stop(self) -> None:
        """Stop the loop."""
        with self._lock:
            self.status = "stopped"
            if self.stop_chan:
                self.stop_chan.set()

    def run(self) -> LoopResponse:
        """Run the loop (non-streaming).

        Returns:
            LoopResponse with all iterations
        """
        self.loop_id = f"loop-{uuid.uuid4().hex[:12]}"
        interval = self._parse_interval(self.request.options.interval)
        max_iterations = self.request.options.max_iterations
        iterations: list[LoopIterationResult] = []
        total_tokens = 0

        while True:
            with self._lock:
                if self.status == "stopped":
                    break
                if max_iterations > 0 and len(iterations) >= max_iterations:
                    self.status = "completed"
                    break

            result = self._run_single_iteration(len(iterations) + 1)
            iterations.append(result)
            total_tokens += result.tokens_used

            if self._check_stop_condition(result):
                result.done = True
                self.status = "completed"
                break

            if interval > 0:
                time.sleep(interval)

        final_content = iterations[-1].content if iterations else ""

        return LoopResponse(
            loop_id=self.loop_id,
            status=self.status,
            iterations=iterations,
            total_tokens=total_tokens,
            total_iterations=len(iterations),
            final_content=final_content,
        )

    def run_streaming(self):
        """Run the loop with streaming.

        Yields:
            StreamEvent objects
        """
        self.loop_id = f"loop-{uuid.uuid4().hex[:12]}"
        interval = self._parse_interval(self.request.options.interval)
        max_iterations = self.request.options.max_iterations
        iterations: list[LoopIterationResult] = []
        total_tokens = 0
        iteration = 0

        while True:
            with self._lock:
                if self.status == "stopped":
                    yield StreamEvent("loop_stopped", {"loop_id": self.loop_id})
                    break
                if max_iterations > 0 and iteration >= max_iterations:
                    self.status = "completed"
                    break

            iteration += 1
            yield StreamEvent("iteration_start", {"iteration": iteration, "loop_id": self.loop_id})

            result = self._run_single_iteration(iteration)
            iterations.append(result)
            total_tokens += result.tokens_used

            yield StreamEvent("iteration_done", {
                "iteration": result.iteration,
                "content": result.content,
                "tool_calls": result.tool_calls,
                "finish_reason": result.finish_reason,
                "tokens_used": result.tokens_used,
                "done": result.done,
            })

            if self._check_stop_condition(result):
                result.done = True
                self.status = "completed"

            if self.status == "completed":
                break

            if interval > 0:
                time.sleep(interval)

        yield StreamEvent("loop_done", {
            "status": self.status,
            "iterations": iteration,
            "total_tokens": total_tokens,
            "final_content": iterations[-1].content if iterations else "",
        })


class LoopRegistry:
    """Global registry for active loops."""

    _loops: dict[str, LoopController] = {}
    _lock = threading.Lock()

    @classmethod
    def register(cls, loop_id: str, controller: LoopController) -> None:
        """Register a loop."""
        with cls._lock:
            cls._loops[loop_id] = controller

    @classmethod
    def unregister(cls, loop_id: str) -> None:
        """Unregister a loop."""
        with cls._lock:
            cls._loops.pop(loop_id, None)

    @classmethod
    def get(cls, loop_id: str) -> Optional[LoopController]:
        """Get a loop by ID."""
        with cls._lock:
            return cls._loops.get(loop_id)

    @classmethod
    def stop(cls, loop_id: str) -> bool:
        """Stop a loop."""
        controller = cls.get(loop_id)
        if controller:
            controller.stop()
            return True
        return False


def create_loop_controller(request: LoopRequest) -> LoopController:
    """Create a new loop controller."""
    return LoopController(request)