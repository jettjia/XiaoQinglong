"""HTTP API routes compatible with Go Runner."""

import uuid
import json
from fastapi import APIRouter, HTTPException
from fastapi.responses import StreamingResponse
from typing import Any

from app.api.schemas import (
    RunRequest,
    RunResponse,
    RunOptions,
    AgentRequest,
    AgentResponse,
    HealthResponse,
    ResumeRequest,
    ResumeResponse,
    ResponseMetadata,
    ToolCall,
    Message,
)
from app.agent.react import create_react_agent
from app.agent.factory import create_llm
from app.agent.checkpoint import get_checkpoint_store, Checkpoint
from app.subagent.manager import SubAgentManager
from app.tools import register_all_tools, get_registry

router = APIRouter()


@router.post("/run", response_model=RunResponse)
async def run(request: RunRequest) -> RunResponse:
    """Run agent with the given request.

    Uses langgraph ReAct agent for tool calling loop.
    """
    try:
        model_config = request.models.get("default")
        if not model_config:
            raise HTTPException(status_code=400, detail="no default model configured")

        options = request.options or RunOptions()

        # Generate checkpoint_id if not provided
        checkpoint_id = request.options.checkpoint_id if request.options else None
        if not checkpoint_id:
            checkpoint_id = f"run-{uuid.uuid4().hex[:12]}"

        # Register all tools
        register_all_tools()
        registry = get_registry()

        # Get langchain tools from registry
        tool_objects = registry.get_langchain_tools()

        # Extract skill IDs and toolsets from request
        skill_ids = [s.id for s in request.skills if s.id]
        toolsets = [s.name for s in request.skills if s.name]

        # Build initial messages
        from langchain_core.messages import HumanMessage, SystemMessage, AIMessage

        messages = []
        if request.system_prompt:
            messages.append(SystemMessage(content=request.system_prompt))
        if request.user_message:
            messages.append(HumanMessage(content=request.user_message))
        for msg in request.messages:
            if msg.role == "user":
                messages.append(HumanMessage(content=msg.content))
            elif msg.role == "assistant":
                messages.append(AIMessage(content=msg.content))

        # Create ReAct agent with tools and skills
        agent = create_react_agent(
            model_config=model_config,
            tools=tool_objects,
            system_prompt=request.system_prompt or "",
            options=options,
            skills=skill_ids if skill_ids else None,
            toolsets=toolsets if toolsets else None,
        )

        # Run the agent
        result = agent.run(messages)

        # Convert tool calls from messages
        tool_calls = []
        for msg in result.messages:
            if hasattr(msg, "tool_calls") and msg.tool_calls:
                for tc in msg.tool_calls:
                    tool_calls.append(ToolCall(
                        tool=tc.get("name", "") or tc.get("function", {}).get("name", ""),
                        input=tc.get("arguments", {}) or tc.get("function", {}).get("arguments", {}),
                        output=None,
                        success=True,
                    ))

        # Save checkpoint
        checkpoint_store = get_checkpoint_store()
        checkpoint_store.create_checkpoint(
            checkpoint_id=checkpoint_id,
            session_id=request.context.get("session_id", checkpoint_id) if request.context else checkpoint_id,
            iteration=result.iterations,
            messages=[
                {"role": m.type if hasattr(m, "type") else str(m.__class__.__name__).lower(),
                 "content": m.content if hasattr(m, "content") else str(m)}
                for m in result.messages
            ],
            tool_results={tc.tool: tc.model_dump() for tc in tool_calls},
            metadata={"model": f"{model_config.provider}/{model_config.name}"},
            finished=result.finished_naturally,
        )

        return RunResponse(
            content=result.output,
            tool_calls=tool_calls,
            finish_reason="stop" if result.finished_naturally else "max_iterations",
            metadata=ResponseMetadata(
                model=f"{model_config.provider}/{model_config.name}",
                iterations=result.iterations,
                tool_calls_count=len(tool_calls),
            ),
            checkpoint_id=checkpoint_id,
        )

    except HTTPException:
        raise
    except Exception as e:
        return RunResponse(
            content="",
            finish_reason="error",
            metadata=ResponseMetadata(
                model="unknown",
                error=str(e),
            ),
        )


@router.post("/run/stream")
async def run_stream(request: RunRequest):
    """Run agent with streaming response.

    Uses Server-Sent Events (SSE) for streaming.
    """
    async def event_generator():
        try:
            model_config = request.models.get("default")
            if not model_config:
                yield f"data: {json.dumps({'error': 'no default model configured'})}\n\n"
                return

            options = request.options or RunOptions()

            # Register tools
            register_all_tools()
            registry = get_registry()

            # Get langchain tools
            tool_objects = registry.get_langchain_tools()

            # Extract skill IDs and toolsets
            skill_ids = [s.id for s in request.skills if s.id]
            toolsets = [s.name for s in request.skills if s.name]

            # Build messages
            from langchain_core.messages import HumanMessage, SystemMessage, AIMessage

            messages = []
            if request.system_prompt:
                messages.append(SystemMessage(content=request.system_prompt))
            if request.user_message:
                messages.append(HumanMessage(content=request.user_message))
            for msg in request.messages:
                if msg.role == "user":
                    messages.append(HumanMessage(content=msg.content))
                elif msg.role == "assistant":
                    messages.append(AIMessage(content=msg.content))

            # Create agent
            agent = create_react_agent(
                model_config=model_config,
                tools=tool_objects,
                system_prompt=request.system_prompt or "",
                options=options,
                skills=skill_ids if skill_ids else None,
                toolsets=toolsets if toolsets else None,
            )

            # Stream results
            for chunk in agent.run_streaming(messages):
                yield f"data: {json.dumps(chunk)}\n\n"

            # Send final event
            yield f"data: {json.dumps({'type': 'done'})}\n\n"

        except Exception as e:
            yield f"data: {json.dumps({'type': 'error', 'error': str(e)})}\n\n"

    return StreamingResponse(
        event_generator(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no",
        },
    )


@router.post("/resume", response_model=ResumeResponse)
async def resume(request: ResumeRequest) -> ResumeResponse:
    """Resume interrupted execution from checkpoint."""
    try:
        checkpoint_store = get_checkpoint_store()
        checkpoint = checkpoint_store.get_checkpoint(request.checkpoint_id)

        if not checkpoint:
            raise HTTPException(status_code=404, detail=f"Checkpoint not found: {request.checkpoint_id}")

        if checkpoint.finished:
            raise HTTPException(status_code=400, detail="Checkpoint already finished")

        # Get model config from metadata or use defaults
        model_config_dict = checkpoint.metadata.get("model", "openai/gpt-4o-mini")
        provider, name = model_config_dict.split("/", 1)

        from app.api.schemas import ModelConfig
        model_config = ModelConfig(
            provider=provider,
            name=name,
            api_key="",  # Would need to be passed or retrieved
        )

        options = RunOptions()

        # Register tools
        register_all_tools()

        # Build messages from checkpoint
        from langchain_core.messages import HumanMessage, SystemMessage, AIMessage, ToolMessage

        messages = []
        for msg_dict in checkpoint.messages:
            role = msg_dict.get("role", "")
            content = msg_dict.get("content", "")
            if role == "system":
                messages.append(SystemMessage(content=content))
            elif role == "user":
                messages.append(HumanMessage(content=content))
            elif role == "assistant":
                messages.append(AIMessage(content=content))
            elif role == "tool":
                messages.append(ToolMessage(content=content, tool_call_id=msg_dict.get("tool_call_id", "")))

        # Create and run agent
        agent = create_react_agent(
            model_config=model_config,
            tools=[],
            system_prompt="",
            options=options,
        )

        result = agent.run(messages)

        # Update checkpoint
        checkpoint_store.create_checkpoint(
            checkpoint_id=request.checkpoint_id,
            session_id=checkpoint.session_id,
            iteration=checkpoint.iteration + result.iterations,
            messages=[
                {"role": m.type if hasattr(m, "type") else str(m.__class__.__name__).lower(),
                 "content": m.content if hasattr(m, "content") else str(m)}
                for m in result.messages
            ],
            tool_results={},
            metadata=checkpoint.metadata,
            finished=result.finished_naturally,
        )

        return ResumeResponse(
            success=True,
            finish_reason="stop" if result.finished_naturally else "max_iterations",
            content=result.output,
            metadata=ResponseMetadata(
                model=model_config_dict,
                iterations=result.iterations,
            ),
        )

    except HTTPException:
        raise
    except Exception as e:
        return ResumeResponse(
            success=False,
            error=str(e),
            finish_reason="error",
        )


@router.post("/stop")
async def stop(request: ResumeRequest):
    """Stop running execution and save checkpoint."""
    try:
        checkpoint_store = get_checkpoint_store()
        checkpoint = checkpoint_store.get_checkpoint(request.checkpoint_id)

        if not checkpoint:
            raise HTTPException(status_code=404, detail=f"Checkpoint not found: {request.checkpoint_id}")

        # Mark as finished
        checkpoint_store.create_checkpoint(
            checkpoint_id=request.checkpoint_id,
            session_id=checkpoint.session_id,
            iteration=checkpoint.iteration,
            messages=checkpoint.messages,
            tool_results=checkpoint.tool_results,
            metadata=checkpoint.metadata,
            finished=True,
        )

        return {"success": True, "message": "Execution stopped"}

    except HTTPException:
        raise
    except Exception as e:
        return {"success": False, "error": str(e)}


@router.get("/checkpoint/{checkpoint_id}")
async def get_checkpoint(checkpoint_id: str):
    """Get checkpoint details."""
    checkpoint_store = get_checkpoint_store()
    checkpoint = checkpoint_store.get_checkpoint(checkpoint_id)

    if not checkpoint:
        raise HTTPException(status_code=404, detail=f"Checkpoint not found: {checkpoint_id}")

    return {
        "checkpoint_id": checkpoint.checkpoint_id,
        "session_id": checkpoint.session_id,
        "created_at": checkpoint.created_at,
        "updated_at": checkpoint.updated_at,
        "iteration": checkpoint.iteration,
        "finished": checkpoint.finished,
        "metadata": checkpoint.metadata,
    }


@router.get("/checkpoints")
async def list_checkpoints():
    """List all checkpoints."""
    checkpoint_store = get_checkpoint_store()
    sessions = checkpoint_store.list_sessions()
    return {"sessions": sessions}


@router.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    """Health check endpoint."""
    return HealthResponse()


@router.get("/agents")
async def list_agents():
    """List available sub-agents."""
    manager = SubAgentManager.get_instance()
    agents = manager.list_agents()
    return {
        "agents": [
            {
                "id": a.id,
                "name": a.name,
                "description": a.description,
            }
            for a in agents
        ]
    }


@router.get("/tools")
async def list_tools():
    """List available tools."""
    register_all_tools()
    registry = get_registry()
    tools = registry.list_tools()
    return {
        "tools": [
            {
                "name": t.name,
                "description": t.description,
            }
            for t in tools
        ]
    }


# ========== Approval Routes ==========

from app.middleware.approval import get_approval_middleware, ApprovalInfo


@router.get("/approvals")
async def list_approvals():
    """List pending approval requests."""
    middleware = get_approval_middleware()
    pending = middleware.get_pending_approvals()
    return {
        "approvals": [
            {
                "approval_id": a.approval_id,
                "tool_name": a.tool_name,
                "tool_type": a.tool_type,
                "arguments": a.arguments,
                "risk_level": a.risk_level,
                "status": a.status.value,
                "created_at": a.created_at,
            }
            for a in pending
        ],
        "count": len(pending),
    }


@router.get("/approvals/{approval_id}")
async def get_approval(approval_id: str):
    """Get approval details."""
    middleware = get_approval_middleware()
    approval = middleware.get_approval(approval_id)

    if not approval:
        raise HTTPException(status_code=404, detail=f"Approval {approval_id} not found")

    return {
        "approval_id": approval.approval_id,
        "tool_name": approval.tool_name,
        "tool_type": approval.tool_type,
        "arguments": approval.arguments,
        "risk_level": approval.risk_level,
        "status": approval.status.value,
        "created_at": approval.created_at,
        "approved_at": approval.approved_at,
        "disapprove_reason": approval.disapprove_reason,
    }


@router.post("/approvals/{approval_id}/approve")
async def approve_tool(approval_id: str):
    """Approve a pending tool execution."""
    middleware = get_approval_middleware()
    if middleware.approve(approval_id):
        return {"success": True, "message": f"Approval {approval_id} approved"}
    raise HTTPException(status_code=404, detail=f"Approval {approval_id} not found")


@router.post("/approvals/{approval_id}/disapprove")
async def disapprove_tool(approval_id: str, reason: str = ""):
    """Disapprove a pending tool execution."""
    middleware = get_approval_middleware()
    if middleware.disapprove(approval_id, reason):
        return {"success": True, "message": f"Approval {approval_id} disapproved"}
    raise HTTPException(status_code=404, detail=f"Approval {approval_id} not found")


@router.post("/approvals/threshold")
async def set_approval_threshold(threshold: str = "medium"):
    """Set the approval threshold (low, medium, high)."""
    middleware = get_approval_middleware()
    middleware.threshold = middleware._parse_risk_level(threshold)
    return {"success": True, "threshold": middleware.threshold.value}


# ========== Loop Routes ==========

from app.agent.loop import LoopController, LoopRequest


@router.post("/loop/start")
async def start_loop(request: dict):
    """Start a continuous loop execution.

    Request body:
    {
        "prompt": "Task to repeat",
        "model": {"provider": "openai", "name": "gpt-4o-mini", ...},
        "options": {
            "max_iterations": 100,
            "interval": "5s",
            "stop_condition": "LOOP_COMPLETE"
        }
    }
    """
    try:
        from app.api.schemas import ModelConfig

        model_config = ModelConfig(**request.get("model", {}))
        options = request.get("options", {})

        from app.agent.loop import LoopOptions
        loop_options = LoopOptions(
            max_iterations=options.get("max_iterations", 100),
            interval=options.get("interval", "5s"),
            stop_condition=options.get("stop_condition", ""),
        )

        from app.agent.loop import LoopRequest as LoopRequestType
        loop_request = LoopRequestType(
            prompt=request.get("prompt", ""),
            model_config=model_config,
            system_prompt=request.get("system_prompt", ""),
            context=request.get("context", {}),
            options=loop_options,
        )

        controller = LoopController(loop_request)
        loop_id = controller.loop_id

        # Run in background
        import threading
        def run_loop():
            result = controller.run()
            logger.info(f"[Loop] {loop_id} completed: {result.status}")

        thread = threading.Thread(target=run_loop, daemon=True)
        thread.start()

        return {
            "success": True,
            "loop_id": loop_id,
            "status": "running",
        }

    except Exception as e:
        logger.error("[loop/start] Failed: %s", e)
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/loop/{loop_id}/stop")
async def stop_loop(loop_id: str):
    """Stop a running loop."""
    from app.agent.loop import LoopRegistry

    if LoopRegistry.stop(loop_id):
        return {"success": True, "message": f"Loop {loop_id} stopped"}
    raise HTTPException(status_code=404, detail=f"Loop {loop_id} not found")


@router.get("/loop/{loop_id}")
async def get_loop_status(loop_id: str):
    """Get loop status."""
    from app.agent.loop import LoopRegistry

    controller = LoopRegistry.get(loop_id)
    if not controller:
        raise HTTPException(status_code=404, detail=f"Loop {loop_id} not found")

    return {
        "loop_id": controller.loop_id,
        "status": controller.status,
        "accumulated_messages": len(controller.accumulated_messages),
    }


# ========== MCP Routes ==========

from app.tools.mcp import MCPToolManager, MCPServerConfig, MCP_AVAILABLE


@router.get("/mcp/servers")
async def list_mcp_servers():
    """List all connected MCP servers."""
    manager = MCPToolManager.get_instance()
    servers = manager.list_servers()
    return {
        "servers": servers,
        "count": len(servers),
        "mcp_available": MCP_AVAILABLE,
    }


@router.get("/mcp/servers/{server_name}")
async def get_mcp_server(server_name: str):
    """Get MCP server details."""
    manager = MCPToolManager.get_instance()
    server = manager.get_server(server_name)

    if not server:
        raise HTTPException(status_code=404, detail=f"Server {server_name} not found")

    return {
        "name": server.config.name,
        "transport": server.config.transport,
        "tools": server.tools,
        "tools_count": len(server.tools),
        "alive": server.alive,
    }


@router.get("/mcp/servers/{server_name}/tools")
async def list_mcp_server_tools(server_name: str):
    """List tools from a specific MCP server."""
    manager = MCPToolManager.get_instance()
    tools = manager.list_tools(server_name)

    if not tools and not manager.get_server(server_name):
        raise HTTPException(status_code=404, detail=f"Server {server_name} not found")

    return {
        "server": server_name,
        "tools": tools,
        "count": len(tools),
    }


@router.post("/mcp/servers")
async def connect_mcp_server(request: dict):
    """Connect to an MCP server.

    Request body:
    {
        "name": "my-server",
        "transport": "stdio",  // or "http"
        "command": "npx",       // for stdio
        "args": ["-y", "@some/mcp-server"],
        "env": {},              // optional env vars
        "url": "http://localhost:8080",  // for http
        "headers": {}           // optional headers for http
    }
    """
    if not MCP_AVAILABLE:
        raise HTTPException(status_code=501, detail="MCP package not installed")

    config = MCPServerConfig(
        name=request.get("name", ""),
        transport=request.get("transport", "stdio"),
        command=request.get("command"),
        args=request.get("args", []),
        env=request.get("env", {}),
        url=request.get("url"),
        headers=request.get("headers", {}),
        timeout=request.get("timeout", 120),
    )

    if not config.name:
        raise HTTPException(status_code=400, detail="Server name is required")

    manager = MCPToolManager.get_instance()

    # Check if already connected
    if manager.get_server(config.name):
        return {
            "success": True,
            "message": f"Server {config.name} already connected",
            "reconnected": False,
        }

    # Connect
    success = manager.connect_server(config)
    if not success:
        raise HTTPException(status_code=500, detail=f"Failed to connect to {config.name}")

    # Register tools
    from app.tools.mcp import register_server_tools
    count = register_server_tools(config.name)

    return {
        "success": True,
        "server": config.name,
        "tools_registered": count,
    }


@router.delete("/mcp/servers/{server_name}")
async def disconnect_mcp_server(server_name: str):
    """Disconnect from an MCP server."""
    manager = MCPToolManager.get_instance()

    if not manager.get_server(server_name):
        raise HTTPException(status_code=404, detail=f"Server {server_name} not found")

    manager.disconnect_server(server_name)

    return {"success": True, "message": f"Server {server_name} disconnected"}


@router.post("/mcp/servers/{server_name}/tools/{tool_name}/call")
async def call_mcp_tool(server_name: str, tool_name: str, request: dict):
    """Call an MCP tool directly.

    Request body: tool arguments as key-value pairs
    """
    manager = MCPToolManager.get_instance()

    if not manager.get_server(server_name):
        raise HTTPException(status_code=404, detail=f"Server {server_name} not found")

    arguments = request.get("arguments", request)

    try:
        result = manager.call_tool(server_name, tool_name, arguments)
        return {"success": True, "result": json.loads(result)}
    except json.JSONDecodeError:
        return {"success": True, "result": result}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
