"""Approval system for dangerous command detection and risk-based authorization.

Based on hermes-agent's approval.py design.
"""

import re
import threading
import uuid
from dataclasses import dataclass, field
from enum import Enum
from typing import Optional

from app.tools.registry import get_registry

logger = __name__


class RiskLevel(Enum):
    """Risk level for commands and tools."""
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"
    CRITICAL = "critical"


# Dangerous command patterns
DANGEROUS_PATTERNS = [
    (r'\brm\s+(-[^\s]*\s*)*[/-rfv]+', "recursive delete"),
    (r'\brm\s+-[^\s]*r', "recursive delete with flag"),
    (r'\bchmod\s+(-[^\s]*\s*)*(777|666|o\+w)', "dangerous permissions"),
    (r'\bchown\s+.*root.*-R', "recursive chown to root"),
    (r'\bmkfs\b', "format filesystem"),
    (r'\bdd\s+.*if=', "disk copy operation"),
    (r'\bsqlite3?\s+.*DROP\s+TABLE', "SQL DROP operation"),
    (r'\bsqlite3?\s+.*DELETE\s+FROM\s+(?!.*WHERE)', "SQL DELETE without WHERE"),
    (r'>\s*/etc/', "write to system config"),
    (r'\bsystemctl\s+(stop|disable|mask)\b', "system service control"),
    (r'\bkill\s+-9\s+-1\b', "kill all processes"),
    (r'\bpkill\s+-9\b', "force kill processes"),
    (r':\(\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;:', "fork bomb"),
    (r'\b(curl|wget)\s+.*\|\s*(ba)?sh', "pipe remote content to shell"),
    (r'\bsudo\s+.*\brm\b', "sudo delete"),
    (r'\brm\s+(-[^\s]*\s*)*-rf\s+/(proc|sys|dev)', "delete system directories"),
    (r'\bcurl\s+.*\$\{', "potential environment variable injection"),
    (r'\bwget\s+.*\$\{', "potential environment variable injection"),
    (r'\beval\s+', "eval command"),
    (r'\bexec\s+', "exec command"),
]


# High-risk tool patterns
HIGH_RISK_PATTERNS = [
    (r'\bsudo\s+', "sudo command"),
    (r'\bchmod\s+777', "chmod 777"),
    (r'\bapt-get\s+install\s+', "package installation"),
    (r'\byum\s+install\s+', "package installation"),
    (r'\bpip\s+install\s+', "pip installation"),
    (r'\bnpm\s+install\s+-g', "global npm installation"),
    (r'\bkill\b', "process kill"),
    (r'\bpkill\b', "process kill"),
    (r'\breboot\b', "system reboot"),
    (r'\bshutdown\b', "system shutdown"),
]


@dataclass
class ApprovalRequest:
    """Represents a pending approval request."""
    interrupt_id: str
    tool_name: str
    tool_type: str
    arguments: str
    risk_level: RiskLevel
    description: str
    session_id: str
    created_at: str = ""
    approved: bool | None = None
    approved_by: str | None = None


class ApprovalStore:
    """Thread-safe store for pending approvals."""

    def __init__(self):
        self._pending: dict[str, ApprovalRequest] = {}
        self._lock = threading.Lock()

    def create(
        self,
        tool_name: str,
        tool_type: str,
        arguments: str,
        risk_level: RiskLevel,
        description: str,
        session_id: str,
    ) -> ApprovalRequest:
        """Create a new pending approval request."""
        from datetime import datetime
        request = ApprovalRequest(
            interrupt_id=str(uuid.uuid4()),
            tool_name=tool_name,
            tool_type=tool_type,
            arguments=arguments,
            risk_level=risk_level,
            description=description,
            session_id=session_id,
            created_at=datetime.utcnow().isoformat(),
        )

        with self._lock:
            self._pending[request.interrupt_id] = request

        return request

    def get(self, interrupt_id: str) -> ApprovalRequest | None:
        """Get a pending approval by ID."""
        with self._lock:
            return self._pending.get(interrupt_id)

    def approve(self, interrupt_id: str, approved_by: str = "user") -> bool:
        """Approve a pending request."""
        with self._lock:
            if interrupt_id in self._pending:
                self._pending[interrupt_id].approved = True
                self._pending[interrupt_id].approved_by = approved_by
                return True
        return False

    def reject(self, interrupt_id: str) -> bool:
        """Reject a pending request."""
        with self._lock:
            if interrupt_id in self._pending:
                self._pending[interrupt_id].approved = False
                return True
        return False

    def list_pending(self, session_id: str | None = None) -> list[ApprovalRequest]:
        """List pending approvals, optionally filtered by session."""
        with self._lock:
            result = [r for r in self._pending.values() if r.approved is None]
            if session_id:
                result = [r for r in result if r.session_id == session_id]
            return result

    def remove(self, interrupt_id: str) -> bool:
        """Remove a pending request."""
        with self._lock:
            if interrupt_id in self._pending:
                del self._pending[interrupt_id]
                return True
        return False


# Global approval store
_approval_store: ApprovalStore | None = None


def get_approval_store() -> ApprovalStore:
    """Get the global approval store."""
    global _approval_store
    if _approval_store is None:
        _approval_store = ApprovalStore()
    return _approval_store


def detect_dangerous_command(command: str) -> tuple[bool, str]:
    """Detect dangerous patterns in shell commands.

    Args:
        command: Shell command string

    Returns:
        (is_dangerous, description)
    """
    for pattern, description in DANGEROUS_PATTERNS:
        if re.search(pattern, command, re.IGNORECASE):
            return True, f"Dangerous pattern detected: {description}"

    for pattern, description in HIGH_RISK_PATTERNS:
        if re.search(pattern, command, re.IGNORECASE):
            return True, f"High-risk pattern: {description}"

    return False, ""


def detect_risk_level(tool_name: str, arguments: dict) -> RiskLevel:
    """Determine risk level for a tool call.

    Args:
        tool_name: Name of the tool
        arguments: Tool arguments

    Returns:
        RiskLevel enum value
    """
    # Check tool name for known risk levels
    high_risk_tools = {
        "bash", "execute_code", "sudo", "chmod", "chown",
        "systemctl", "kill", "pkill", "reboot", "shutdown",
    }
    medium_risk_tools = {
        "file_write", "file_delete", "sql_execute",
        "network_request", "download",
    }

    if tool_name in high_risk_tools:
        return RiskLevel.HIGH

    if tool_name in medium_risk_tools:
        return RiskLevel.MEDIUM

    # Check bash commands for dangerous patterns
    if tool_name == "bash":
        command = arguments.get("command", "")
        is_dangerous, _ = detect_dangerous_command(command)
        if is_dangerous:
            return RiskLevel.HIGH

    return RiskLevel.LOW


def check_approval_required(
    tool_name: str,
    arguments: dict,
    risk_threshold: RiskLevel = RiskLevel.MEDIUM,
    auto_approve: list[str] | None = None,
) -> tuple[bool, ApprovalRequest | None]:
    """Check if approval is required for a tool call.

    Args:
        tool_name: Name of the tool
        arguments: Tool arguments
        risk_threshold: Minimum risk level requiring approval
        auto_approve: List of tool names to auto-approve

    Returns:
        (requires_approval, approval_request_or_None)
    """
    # Auto-approve whitelist
    if auto_approve and tool_name in auto_approve:
        return False, None

    # Determine risk level
    risk_level = detect_risk_level(tool_name, arguments)

    # Compare against threshold
    risk_order = {RiskLevel.LOW: 0, RiskLevel.MEDIUM: 1, RiskLevel.HIGH: 2, RiskLevel.CRITICAL: 3}

    if risk_order[risk_level] < risk_order[risk_threshold]:
        return False, None

    # Create approval request
    store = get_approval_store()
    request = store.create(
        tool_name=tool_name,
        tool_type="bash" if tool_name == "bash" else "tool",
        arguments=str(arguments),
        risk_level=risk_level,
        description=f"Tool '{tool_name}' requires approval (risk: {risk_level.value})",
        session_id="default",
    )

    return True, request


def auto_approve_safe_tools(tool_name: str, arguments: dict) -> bool:
    """Check if a tool call can be auto-approved.

    Args:
        tool_name: Name of the tool
        arguments: Tool arguments

    Returns:
        True if safe to auto-approve
    """
    risk = detect_risk_level(tool_name, arguments)
    return risk == RiskLevel.LOW


def format_approval_message(request: ApprovalRequest) -> str:
    """Format an approval request as a readable message."""
    return f"""
Approval Required
==================
ID: {request.interrupt_id}
Tool: {request.tool_name}
Type: {request.tool_type}
Risk: {request.risk_level.value.upper()}
Arguments: {request.arguments}

{request.description}

To approve: POST /resume with approval
"""
