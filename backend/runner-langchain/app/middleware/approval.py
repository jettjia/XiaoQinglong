"""Approval middleware for high-risk tool execution.

Based on Go runner's approval_middleware.go design.
Intercepts high-risk tool calls and requires human approval.
"""

import json
import logging
import threading
import uuid
from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Any, Optional

logger = logging.getLogger(__name__)


class RiskLevel(str, Enum):
    """Tool risk levels."""
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"


class ApprovalStatus(str, Enum):
    """Approval status."""
    PENDING = "pending"
    APPROVED = "approved"
    DISAPPROVED = "disapproved"


# Default threshold - tools at or above this level require approval
DEFAULT_APPROVAL_THRESHOLD = RiskLevel.MEDIUM

# Risk levels for built-in tools
TOOL_RISK_LEVELS = {
    "bash": RiskLevel.HIGH,
    "file_write": RiskLevel.HIGH,
    "edit": RiskLevel.MEDIUM,
    "execute_skill_script_file": RiskLevel.MEDIUM,
    "delegate_to_agent": RiskLevel.MEDIUM,
    "sleep": RiskLevel.LOW,
    "parallel": RiskLevel.LOW,
    "result_limiter": RiskLevel.LOW,
    "web_search": RiskLevel.LOW,
    "web_fetch": RiskLevel.LOW,
    "glob": RiskLevel.LOW,
    "grep": RiskLevel.LOW,
    "file_read": RiskLevel.LOW,
    "skills_list": RiskLevel.LOW,
    "skill_view": RiskLevel.LOW,
    "skill_create": RiskLevel.MEDIUM,
    "skill_patch": RiskLevel.MEDIUM,
    "skill_delete": RiskLevel.HIGH,
    "load_skill": RiskLevel.LOW,
    "html_interpreter": RiskLevel.LOW,
    "memory": RiskLevel.LOW,
    "task_create": RiskLevel.MEDIUM,
    "task_update": RiskLevel.MEDIUM,
    "task_delete": RiskLevel.MEDIUM,
    "todo_write": RiskLevel.LOW,
    "plan_mode": RiskLevel.LOW,
    "question": RiskLevel.LOW,
    "cron_create": RiskLevel.MEDIUM,
    "cron_delete": RiskLevel.MEDIUM,
    "cron_list": RiskLevel.LOW,
}


@dataclass
class ApprovalInfo:
    """Information about a pending approval."""
    approval_id: str
    tool_name: str
    tool_type: str  # http, mcp, skill, a2a
    arguments: str  # JSON string
    risk_level: str
    description: str = ""
    status: ApprovalStatus = ApprovalStatus.PENDING
    created_at: str = ""
    approved_at: Optional[str] = None
    disapprove_reason: Optional[str] = None

    def __post_init__(self):
        if not self.created_at:
            self.created_at = datetime.utcnow().isoformat()


@dataclass
class ApprovalResult:
    """Result of an approval decision."""
    approved: bool
    disapprove_reason: Optional[str] = None


class ApprovalMiddleware:
    """Middleware that intercepts high-risk tool calls for approval.

    Based on Go runner's approval middleware design.
    """

    def __init__(
        self,
        tool_risk_levels: Optional[dict[str, str]] = None,
        threshold: str = "medium",
    ):
        self.tool_risk_levels = tool_risk_levels or TOOL_RISK_LEVELS.copy()
        self.threshold = self._parse_risk_level(threshold)
        self._pending_approvals: dict[str, ApprovalInfo] = {}
        self._lock = threading.Lock()
        self._approval_callbacks: list[callable] = []

    def _parse_risk_level(self, level: str) -> RiskLevel:
        """Parse risk level string."""
        try:
            return RiskLevel(level.lower())
        except ValueError:
            return RiskLevel.MEDIUM

    def _get_risk_level(self, tool_name: str) -> RiskLevel:
        """Get risk level for a tool."""
        level_str = self.tool_risk_levels.get(tool_name.lower(), "low")
        return self._parse_risk_level(level_str)

    def _should_approve(self, tool_name: str) -> bool:
        """Check if tool requires approval based on risk level."""
        risk = self._get_risk_level(tool_name)
        risk_values = {RiskLevel.LOW: 1, RiskLevel.MEDIUM: 2, RiskLevel.HIGH: 3}
        return risk_values.get(risk, 0) >= risk_values.get(self.threshold, 0)

    def add_approval_callback(self, callback: callable) -> None:
        """Add callback for new approval requests."""
        with self._lock:
            self._approval_callbacks.append(callback)

    def request_approval(
        self,
        tool_name: str,
        arguments: Any,
        tool_type: str = "http",
    ) -> Optional[str]:
        """Request approval for a tool call.

        Returns approval_id if approval is needed, None otherwise.
        """
        if not self._should_approve(tool_name):
            return None

        arguments_json = json.dumps(arguments) if isinstance(arguments, dict) else str(arguments)

        approval_info = ApprovalInfo(
            approval_id=str(uuid.uuid4())[:8],
            tool_name=tool_name,
            tool_type=tool_type,
            arguments=arguments_json,
            risk_level=self._get_risk_level(tool_name).value,
        )

        with self._lock:
            self._pending_approvals[approval_info.approval_id] = approval_info

        # Notify callbacks
        for callback in self._approval_callbacks:
            try:
                callback(approval_info)
            except Exception as e:
                logger.warning("[ApprovalMiddleware] Callback failed: %s", e)

        logger.info("[ApprovalMiddleware] Approval requested for %s (risk=%s)",
                    tool_name, approval_info.risk_level)

        return approval_info.approval_id

    def get_pending_approvals(self) -> list[ApprovalInfo]:
        """Get all pending approvals."""
        with self._lock:
            return [a for a in self._pending_approvals.values()
                    if a.status == ApprovalStatus.PENDING]

    def get_approval(self, approval_id: str) -> Optional[ApprovalInfo]:
        """Get approval by ID."""
        with self._lock:
            return self._pending_approvals.get(approval_id)

    def approve(self, approval_id: str) -> bool:
        """Approve a pending request."""
        with self._lock:
            if approval_id not in self._pending_approvals:
                return False

            approval = self._pending_approvals[approval_id]
            approval.status = ApprovalStatus.APPROVED
            approval.approved_at = datetime.utcnow().isoformat()

            logger.info("[ApprovalMiddleware] Approved %s", approval_id)
            return True

    def disapprove(self, approval_id: str, reason: Optional[str] = None) -> bool:
        """Disapprove a pending request."""
        with self._lock:
            if approval_id not in self._pending_approvals:
                return False

            approval = self._pending_approvals[approval_id]
            approval.status = ApprovalStatus.DISAPPROVED
            approval.approved_at = datetime.utcnow().isoformat()
            approval.disapprove_reason = reason

            logger.info("[ApprovalMiddleware] Disapproved %s: %s", approval_id, reason)
            return True

    def get_approval_result(self, approval_id: str) -> Optional[ApprovalResult]:
        """Get approval result for resume."""
        with self._lock:
            approval = self._pending_approvals.get(approval_id)
            if not approval:
                return None

            if approval.status == ApprovalStatus.APPROVED:
                return ApprovalResult(approved=True)
            elif approval.status == ApprovalStatus.DISAPPROVED:
                return ApprovalResult(
                    approved=False,
                    disapprove_reason=approval.disapprove_reason,
                )
            return None


class ApprovalStore:
    """Global store for approvals."""
    _instance: Optional[ApprovalMiddleware] = None
    _lock = threading.Lock()

    @classmethod
    def get_instance(cls) -> ApprovalMiddleware:
        if cls._instance is None:
            with cls._lock:
                if cls._instance is None:
                    cls._instance = ApprovalMiddleware()
        return cls._instance

    @classmethod
    def set_threshold(cls, threshold: str) -> None:
        instance = cls.get_instance()
        instance.threshold = instance._parse_risk_level(threshold)


def get_approval_middleware() -> ApprovalMiddleware:
    """Get the global approval middleware."""
    return ApprovalStore.get_instance()


def set_tool_risk_level(tool_name: str, level: str) -> None:
    """Set risk level for a tool."""
    middleware = get_approval_middleware()
    middleware.tool_risk_levels[tool_name.lower()] = level.lower()


def should_approve(tool_name: str) -> bool:
    """Check if tool requires approval."""
    return get_approval_middleware()._should_approve(tool_name)