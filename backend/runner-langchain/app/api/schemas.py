"""Pydantic schemas for API requests and responses - Go Runner compatible."""

from pydantic import BaseModel, Field
from typing import Any, Optional


class ModelConfig(BaseModel):
    """Model configuration."""

    provider: str = Field(description="Model provider (openai, anthropic, etc.)")
    name: str = Field(description="Model name")
    api_key: str = Field(description="API key")
    api_base: Optional[str] = Field(default=None, description="API base URL")
    temperature: float = Field(default=0.7, ge=0.0, le=2.0)
    max_tokens: int = Field(default=2000, ge=1)
    top_p: Optional[float] = Field(default=None, ge=0.0, le=1.0)
    extra_fields: Optional[dict[str, Any]] = Field(default=None, description="Extra parameters")


class RetryConfig(BaseModel):
    """Retry configuration."""

    max_attempts: int = Field(default=3)
    initial_delay_ms: int = Field(default=1000)
    max_delay_ms: int = Field(default=10000)
    backoff_multiplier: float = Field(default=2.0)
    retryable_errors: list[str] = Field(default_factory=list)
    fallback_model: Optional[str] = Field(default=None)
    circuit_breaker_threshold: int = Field(default=0)
    circuit_breaker_duration_ms: int = Field(default=0)


class RoutingConfig(BaseModel):
    """Multi-model routing configuration."""

    default_model: str = Field(default="default")
    rewrite_prompt: Optional[str] = None
    summarize_prompt: Optional[str] = None


class ApprovalPolicy(BaseModel):
    """Approval policy for tool calls."""

    enabled: bool = Field(default=False)
    risk_threshold: str = Field(default="medium")
    auto_approve: list[str] = Field(default_factory=list)


class ResponseSchemaConfig(BaseModel):
    """Response schema configuration."""

    type: str = Field(default="text")
    version: str = Field(default="1.0")
    strict: bool = Field(default=True)
    json_schema: Optional[dict[str, Any]] = Field(default=None, alias="schema")
    fallback: Optional[str] = None


class RunOptions(BaseModel):
    """Runtime options for agent execution."""

    temperature: float = Field(default=0.7, ge=0.0, le=2.0)
    max_tokens: int = Field(default=2000, ge=1)
    max_iterations: int = Field(default=50, ge=1)
    max_tool_calls: int = Field(default=30, ge=1)
    max_a2a_calls: int = Field(default=10, ge=1)
    max_total_tokens: int = Field(default=100000, ge=1)
    stream: bool = Field(default=False)
    stop: Optional[list[str]] = Field(default=None)
    timeout_ms: int = Field(default=60000, ge=1000)
    retry: Optional[RetryConfig] = None
    routing: Optional[RoutingConfig] = None
    approval_policy: Optional[ApprovalPolicy] = None
    checkpoint_id: Optional[str] = None
    response_schema: Optional[ResponseSchemaConfig] = None
    loop_interval: Optional[str] = None
    loop_max_iterations: int = Field(default=0)
    loop_stop_condition: Optional[str] = None


class DelegateTask(BaseModel):
    """Single delegated task."""

    goal: str = Field(description="Task description")
    context: Optional[str] = Field(default=None, description="Additional context")
    tools: list[str] = Field(default_factory=list, description="Allowed tools")
    workspace: Optional[str] = Field(default=None, description="Working directory")


class DelegateInput(BaseModel):
    """Input for delegate_to_agent tool."""

    agent_id: Optional[str] = Field(default=None, description="Sub-agent ID")
    task: Optional[str] = Field(default=None, description="Single task (legacy mode)")
    tasks: Optional[list[DelegateTask]] = Field(default=None, description="Batch tasks")
    context: Optional[str] = Field(default=None, description="Additional context")
    depth: int = Field(default=0, description="Current delegation depth")


class Message(BaseModel):
    """Chat message."""

    role: str = Field(description="Message role (user, assistant, system)")
    content: str = Field(description="Message content")


class ToolConfig(BaseModel):
    """Tool configuration."""

    type: str = Field(description="Tool type (http, mcp, etc.)")
    name: str = Field(description="Tool name")
    description: Optional[str] = Field(default=None)
    endpoint: Optional[str] = Field(default=None)
    method: Optional[str] = Field(default=None)
    headers: Optional[dict[str, str]] = Field(default=None)
    risk_level: Optional[str] = Field(default=None)


class MCPConfig(BaseModel):
    """MCP server configuration."""

    name: str
    transport: str = Field(default="stdio")
    command: Optional[str] = None
    args: list[str] = Field(default_factory=list)
    env: dict[str, str] = Field(default_factory=dict)
    endpoint: Optional[str] = None
    headers: Optional[dict[str, str]] = None
    risk_level: Optional[str] = None


class A2AAgentConfig(BaseModel):
    """A2A agent configuration."""

    name: str
    endpoint: str
    headers: dict[str, str] = Field(default_factory=dict)
    risk_level: Optional[str] = None


class CLIConfig(BaseModel):
    """CLI tool configuration."""

    name: str
    command: str
    config_dir: Optional[str] = None
    skills_dir: Optional[str] = None
    risk_level: Optional[str] = None
    auth_type: Optional[str] = None


class SubAgentConfig(BaseModel):
    """Sub-agent configuration."""

    id: str = Field(description="Agent ID")
    name: str = Field(description="Agent name")
    description: Optional[str] = Field(default=None)
    prompt: str = Field(description="System prompt")
    model: ModelConfig
    tools: list[str] = Field(default_factory=list)
    skills: list[str] = Field(default_factory=list)
    max_iterations: int = Field(default=50)
    timeout_ms: int = Field(default=30000)


class SandboxLimits(BaseModel):
    """Sandbox resource limits."""

    cpu: Optional[str] = None
    memory: Optional[str] = None


class VolumeMount(BaseModel):
    """Volume mount configuration."""

    host_path: str
    container_path: str
    read_only: bool = False


class SandboxConfig(BaseModel):
    """Sandbox configuration."""

    enabled: bool = False
    mode: str = Field(default="docker")
    image: Optional[str] = None
    workdir: Optional[str] = None
    network: Optional[str] = None
    timeout_ms: int = Field(default=120000)
    env: dict[str, str] = Field(default_factory=dict)
    limits: Optional[SandboxLimits] = None
    volumes: list[VolumeMount] = Field(default_factory=list)


class KnowledgeBaseConfig(BaseModel):
    """Knowledge base configuration."""

    id: str
    name: str
    retrieval_url: str
    token: Optional[str] = None
    top_k: int = Field(default=5)


class FileConfig(BaseModel):
    """File configuration."""

    name: str
    virtual_path: Optional[str] = None
    size: int = 0
    type: Optional[str] = None


class Skill(BaseModel):
    """Skill configuration."""

    id: str
    name: Optional[str] = None
    description: Optional[str] = None
    instruction: Optional[str] = None
    scope: Optional[str] = None
    trigger: Optional[str] = None
    entry_script: Optional[str] = None
    file_path: Optional[str] = None
    inputs: list[str] = Field(default_factory=list)
    outputs: list[str] = Field(default_factory=list)
    risk_level: Optional[str] = None
    output_patterns: list[str] = Field(default_factory=list)


class RunRequest(BaseModel):
    """Run request (compatible with Go Runner)."""

    endpoint: Optional[str] = Field(default=None, description="Target endpoint (ignored)")
    models: dict[str, ModelConfig] = Field(description="Model configurations")
    system_prompt: str = Field(description="System prompt")
    user_message: Optional[str] = Field(default=None, description="User message")
    messages: list[Message] = Field(default_factory=list)
    context: dict[str, Any] = Field(default_factory=dict)
    knowledge_bases: list[KnowledgeBaseConfig] = Field(default_factory=list)
    skills: list[Skill] = Field(default_factory=list)
    mcps: list[MCPConfig] = Field(default_factory=list)
    clis: list[CLIConfig] = Field(default_factory=list)
    a2a: list[A2AAgentConfig] = Field(default_factory=list)
    tools: list[ToolConfig] = Field(default_factory=list)
    internal_agents: list[dict] = Field(default_factory=list)
    sub_agents: list[SubAgentConfig] = Field(default_factory=list)
    options: Optional[RunOptions] = Field(default=None)
    sandbox: Optional[SandboxConfig] = Field(default=None)
    files: list[FileConfig] = Field(default_factory=list)


class ToolCall(BaseModel):
    """Tool call record."""

    tool: str = Field(description="Tool name")
    input: dict[str, Any] = Field(description="Tool input")
    output: Optional[str] = Field(default=None)
    success: bool = Field(default=True)
    error: Optional[str] = None


class A2AResult(BaseModel):
    """A2A result."""

    agent_name: str
    status: str
    result: Optional[Any] = None
    error: Optional[str] = None


class PendingApproval(BaseModel):
    """Pending approval information."""

    interrupt_id: str
    tool_name: str
    tool_type: str
    arguments_json: str
    risk_level: str
    description: Optional[str] = None


class MemoryEntry(BaseModel):
    """Memory entry."""

    name: str
    description: str
    type: str
    content: str
    importance: int = 0


class ToolCallMetadata(BaseModel):
    """Tool call metadata."""

    tool: str
    input: Optional[Any] = None
    output: Optional[Any] = None
    latency_ms: int = 0
    success: bool = True
    error: Optional[str] = None


class ResponseMetadata(BaseModel):
    """Response metadata."""

    model: str = ""
    latency_ms: int = 0
    tokens_used: int = 0
    prompt_tokens: int = 0
    completion_tokens: int = 0
    tool_calls_count: int = 0
    a2a_calls_count: int = 0
    skill_calls_count: int = 0
    iterations: int = 0
    tool_calls_detail: list[ToolCallMetadata] = Field(default_factory=list)
    error: Optional[str] = None


class RunResponse(BaseModel):
    """Run response (compatible with Go Runner)."""

    content: str = Field(default="")
    tool_calls: list[ToolCall] = Field(default_factory=list)
    a2a_results: list[A2AResult] = Field(default_factory=list)
    tokens_used: int = 0
    finish_reason: str = Field(default="stop")
    metadata: ResponseMetadata
    a2ui_messages: list[Any] = Field(default_factory=list)
    pending_approvals: list[PendingApproval] = Field(default_factory=list)
    checkpoint_id: Optional[str] = None
    memories: list[MemoryEntry] = Field(default_factory=list)


class AgentRequest(BaseModel):
    """Agent autonomous execution request."""

    task: str = Field(description="Natural language task")
    context: dict[str, Any] = Field(default_factory=dict)
    models: Optional[dict[str, ModelConfig]] = None
    stream: bool = Field(default=False)


class AgentResponse(BaseModel):
    """Agent execution response."""

    content: str = Field(default="")
    tool_calls: list[ToolCall] = Field(default_factory=list)
    tokens_used: int = 0
    finish_reason: str = Field(default="stop")
    metadata: ResponseMetadata
    error: Optional[str] = None


class HealthResponse(BaseModel):
    """Health check response."""

    status: str = "ok"
    port: int = 18088
    version: str = "0.1.0"


class ResumeApproval(BaseModel):
    """Resume approval."""

    interrupt_id: str
    approved: bool
    disapprove_reason: Optional[str] = None


class ResumeRequest(BaseModel):
    """Resume request."""

    checkpoint_id: str
    approvals: list[ResumeApproval] = Field(default_factory=list)


class ResumeResponse(BaseModel):
    """Resume response."""

    success: bool = True
    error: Optional[str] = None
    finish_reason: str = "stop"
    content: str = ""
    tool_calls: list[ToolCall] = Field(default_factory=list)
    metadata: ResponseMetadata
