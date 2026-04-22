"""Plan Mode tools for planning before implementation.

Based on Go runner's plan_mode.go design.
Provides EnterPlanMode and ExitPlanMode tools.
"""

import json
import threading
from typing import Optional

from app.tools.registry import get_registry


class PlanModeState:
    """Thread-safe plan mode state tracker."""

    _instance: Optional["PlanModeState"] = None
    _lock = threading.Lock()

    def __init__(self):
        self.in_plan_mode = False
        self.phase = ""
        self._lock = threading.Lock()

    @classmethod
    def get_instance(cls) -> "PlanModeState":
        """Get singleton instance."""
        if cls._instance is None:
            with cls._lock:
                if cls._instance is None:
                    cls._instance = cls()
        return cls._instance

    def enter(self) -> bool:
        """Enter plan mode. Returns False if already in plan mode."""
        with self._lock:
            if self.in_plan_mode:
                return False
            self.in_plan_mode = True
            self.phase = "planning"
            return True

    def exit(self) -> bool:
        """Exit plan mode. Returns False if not in plan mode."""
        with self._lock:
            if not self.in_plan_mode:
                return False
            self.in_plan_mode = False
            self.phase = ""
            return True

    def is_in_plan_mode(self) -> bool:
        """Check if in plan mode."""
        with self._lock:
            return self.in_plan_mode

    def set_phase(self, phase: str) -> None:
        """Set the current phase."""
        with self._lock:
            self.phase = phase

    def get_phase(self) -> str:
        """Get current phase."""
        with self._lock:
            return self.phase


# Global state
_plan_mode_state: Optional[PlanModeState] = None


def get_plan_mode_state() -> PlanModeState:
    """Get the global plan mode state."""
    global _plan_mode_state
    if _plan_mode_state is None:
        _plan_mode_state = PlanModeState.get_instance()
    return _plan_mode_state


def _enter_plan_mode(reason: str = "") -> str:
    """Enter plan mode for implementation planning.

    Args:
        reason: Reason for entering plan mode

    Returns:
        JSON with success status and message
    """
    if not reason:
        reason = "Planning implementation approach"

    state = get_plan_mode_state()

    if not state.enter():
        return json.dumps({
            "success": False,
            "message": "Already in plan mode",
        })

    return json.dumps({
        "success": True,
        "message": f"Entered plan mode. {reason}",
    })


def _exit_plan_mode(approved: bool, feedback: str = "") -> str:
    """Exit plan mode with approval.

    Args:
        approved: Whether the plan is approved
        feedback: User feedback on the plan

    Returns:
        JSON with success status, message, and can_proceed flag
    """
    state = get_plan_mode_state()

    if not state.exit():
        return json.dumps({
            "success": False,
            "message": "Not in plan mode",
            "can_proceed": False,
        })

    if approved:
        message = "Plan approved, ready to proceed"
    else:
        message = "Plan not approved"

    if feedback:
        message += f". Feedback: {feedback}"

    return json.dumps({
        "success": True,
        "message": message,
        "can_proceed": approved,
    })


def _get_plan_mode_status() -> str:
    """Get current plan mode status."""
    state = get_plan_mode_state()
    return json.dumps({
        "in_plan_mode": state.is_in_plan_mode(),
        "phase": state.get_phase(),
    })


def register_tools() -> None:
    """Register plan mode tools."""
    registry = get_registry()

    # EnterPlanMode
    registry.register(
        name="enter_plan_mode",
        description="Enter plan mode for implementation planning before coding. Use this when you need to design an approach or get user approval before proceeding.",
        schema={
            "type": "object",
            "properties": {
                "reason": {
                    "type": "string",
                    "description": "Reason for entering plan mode",
                },
            },
            "required": [],
        },
        handler=lambda **kwargs: _enter_plan_mode(reason=kwargs.get("reason", "")),
    )

    # ExitPlanMode
    registry.register(
        name="exit_plan_mode",
        description="Exit plan mode with user approval. Call this when the plan is ready for implementation.",
        schema={
            "type": "object",
            "properties": {
                "approved": {
                    "type": "boolean",
                    "description": "Whether the plan is approved to proceed",
                },
                "feedback": {
                    "type": "string",
                    "description": "User feedback on the plan",
                },
            },
            "required": ["approved"],
        },
        handler=lambda **kwargs: _exit_plan_mode(
            approved=kwargs.get("approved", False),
            feedback=kwargs.get("feedback", ""),
        ),
    )