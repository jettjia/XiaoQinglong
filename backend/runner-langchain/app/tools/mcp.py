"""MCP (Model Context Protocol) tool support.

Supports:
- Multiple MCP servers
- Stdio transport (command + args)
- HTTP/SSE transport (url + headers)
- Tool discovery and execution
"""

import asyncio
import json
import logging
import threading
from dataclasses import dataclass, field
from typing import Any, Optional

from app.tools.registry import get_registry

logger = logging.getLogger(__name__)

# MCP package availability
MCP_AVAILABLE = False
try:
    from mcp import ClientSession
    from mcp.client.stdio import stdio_client
    from mcp.client.streamable_http import streamablehttp_client
    MCP_AVAILABLE = True
except ImportError:
    logger.warning("MCP package not installed. Run: uv add mcp")


@dataclass
class MCPServerConfig:
    """MCP server configuration."""
    name: str
    transport: str = "stdio"  # "stdio" or "http"
    command: Optional[str] = None
    args: list[str] = field(default_factory=list)
    env: dict[str, str] = field(default_factory=dict)
    url: Optional[str] = None
    headers: dict[str, str] = field(default_factory=dict)
    timeout: int = 120


@dataclass
class MCPServer:
    """Connected MCP server."""
    config: MCPServerConfig
    session: Any = None
    tools: list[dict] = field(default_factory=list)
    alive: bool = False


class MCPToolManager:
    """Manages multiple MCP server connections and tool execution.

    Based on hermes-agent's mcp_tool.py design.
    Supports multiple concurrent servers with auto-reconnection.
    """

    _instance: Optional["MCPToolManager"] = None
    _lock = threading.Lock()

    def __init__(self):
        self._servers: dict[str, MCPServer] = {}
        self._loop: Optional[asyncio.AbstractEventLoop] = None
        self._thread: Optional[threading.Thread] = None
        self._pending_connections: dict[str, asyncio.Future] = {}

    @classmethod
    def get_instance(cls) -> "MCPToolManager":
        if cls._instance is None:
            with cls._lock:
                if cls._instance is None:
                    cls._instance = cls()
        return cls._instance

    def start_background_loop(self) -> None:
        """Start the background asyncio event loop."""
        if self._loop is not None and not self._loop.is_closed():
            return

        def run_loop():
            self._loop = asyncio.new_event_loop()
            asyncio.set_event_loop(self._loop)
            self._loop.run_forever()

        self._thread = threading.Thread(target=run_loop, daemon=True)
        self._thread.start()

        import time
        time.sleep(0.1)

    def _ensure_loop(self) -> asyncio.AbstractEventLoop:
        """Ensure the event loop is running."""
        if self._loop is None or self._loop.is_closed():
            self.start_background_loop()
        return self._loop

    async def _connect_stdio(self, config: MCPServerConfig) -> tuple[Any, list]:
        """Connect to MCP server via stdio transport."""
        from mcp.client.stdio import stdio_client
        from mcp import StdioServerParameters

        server_params = StdioServerParameters(
            command=config.command or "npx",
            args=config.args,
            env=config.env,
        )

        async with stdio_client(server_params) as (read, write):
            session = ClientSession(read, write)
            await session.initialize()
            tools = await session.list_tools()
            return session, tools

    async def _connect_http(self, config: MCPServerConfig) -> tuple[Any, list]:
        """Connect to MCP server via HTTP/SSE transport."""
        from mcp.client.streamable_http import streamablehttp_client

        async with streamablehttp_client(
            config.url,
            headers=config.headers,
        ) as (read, write, get_session):
            session = await get_session()
            tools = await session.list_tools()
            return session, tools

    async def connect_server_async(self, config: MCPServerConfig) -> bool:
        """Connect to an MCP server and discover its tools (async)."""
        if not MCP_AVAILABLE:
            logger.error("MCP package not installed")
            return False

        try:
            if config.transport == "stdio":
                session, tools_result = await self._connect_stdio(config)
            elif config.transport == "http":
                session, tools_result = await self._connect_http(config)
            else:
                logger.error(f"Unknown MCP transport: {config.transport}")
                return False

            server = MCPServer(
                config=config,
                session=session,
                tools=[
                    {
                        "name": t.name,
                        "description": t.description,
                        "input_schema": t.inputSchema,
                    }
                    for t in tools_result.tools
                ],
                alive=True,
            )

            with self._lock:
                self._servers[config.name] = server

            logger.info(f"Connected to MCP server: {config.name} with {len(server.tools)} tools")
            return True

        except Exception as e:
            logger.error(f"Failed to connect to MCP server {config.name}: {e}")
            return False

    def connect_server(self, config: MCPServerConfig, timeout: int = 60) -> bool:
        """Connect to an MCP server synchronously."""
        loop = self._ensure_loop()

        try:
            future = asyncio.run_coroutine_threadsafe(
                self.connect_server_async(config),
                loop,
            )
            return future.result(timeout=timeout)
        except Exception as e:
            logger.error(f"connect_server failed: {e}")
            return False

    async def disconnect_server_async(self, name: str) -> None:
        """Disconnect from an MCP server (async)."""
        with self._lock:
            if name not in self._servers:
                return

            server = self._servers[name]
            self._servers.pop(name, None)

        if server.session:
            try:
                await server.session.close()
            except Exception as e:
                logger.warning(f"Error closing MCP session: {e}")

    def disconnect_server(self, name: str) -> bool:
        """Disconnect from an MCP server."""
        loop = self._ensure_loop()

        try:
            future = asyncio.run_coroutine_threadsafe(
                self.disconnect_server_async(name),
                loop,
            )
            future.result(timeout=10)
            return True
        except Exception as e:
            logger.error(f"disconnect_server failed: {e}")
            return False

    async def call_tool_async(self, server_name: str, tool_name: str, arguments: dict) -> str:
        """Call a tool on an MCP server (async)."""
        with self._lock:
            if server_name not in self._servers:
                return json.dumps({"error": f"Server not found: {server_name}"})
            server = self._servers[server_name]

        if not server.alive:
            return json.dumps({"error": f"Server not alive: {server_name}"})

        try:
            result = await server.session.call_tool(tool_name, arguments)
            content = []
            if result.content:
                for item in result.content:
                    if hasattr(item, 'text'):
                        content.append({"type": "text", "text": item.text})
                    elif hasattr(item, 'data'):
                        content.append({"type": "text", "text": str(item.data)})

            return json.dumps({
                "content": content,
                "isError": result.isError,
            })
        except Exception as e:
            logger.error(f"MCP tool call failed: {e}")
            return json.dumps({"error": str(e)})

    def call_tool(self, server_name: str, tool_name: str, arguments: dict) -> str:
        """Call a tool on an MCP server synchronously."""
        loop = self._ensure_loop()

        try:
            future = asyncio.run_coroutine_threadsafe(
                self.call_tool_async(server_name, tool_name, arguments),
                loop,
            )
            return future.result(timeout=120)
        except Exception as e:
            logger.error(f"call_tool failed: {e}")
            return json.dumps({"error": str(e)})

    def list_servers(self) -> list[dict]:
        """List all connected MCP servers."""
        with self._lock:
            servers = list(self._servers.values())

        return [
            {
                "name": s.config.name,
                "transport": s.config.transport,
                "tools_count": len(s.tools),
                "alive": s.alive,
            }
            for s in servers
        ]

    def list_tools(self, server_name: Optional[str] = None) -> list[dict]:
        """List tools from MCP servers."""
        with self._lock:
            if server_name:
                if server_name in self._servers:
                    return self._servers[server_name].tools.copy()
                return []

            tools = []
            for server in self._servers.values():
                for tool in server.tools:
                    tools.append({
                        **tool,
                        "server": server.config.name,
                    })
            return tools

    def get_server(self, name: str) -> Optional[MCPServer]:
        """Get a server by name."""
        with self._lock:
            return self._servers.get(name)

    def get_all_configs(self) -> list[MCPServerConfig]:
        """Get all server configurations."""
        with self._lock:
            return [s.config for s in self._servers.values()]


def _mcp_handler(server_name: str, tool_name: str):
    """Create an async handler for MCP tool calls."""
    async def handler(**kwargs) -> str:
        manager = MCPToolManager.get_instance()
        return await manager.call_tool_async(server_name, tool_name, kwargs)
    return handler


def register_mcp_tools(servers: list[MCPServerConfig]) -> None:
    """Register MCP server tools in the tool registry.

    Args:
        servers: List of MCP server configurations
    """
    if not MCP_AVAILABLE:
        logger.warning("MCP not available, skipping tool registration")
        return

    manager = MCPToolManager.get_instance()
    manager.start_background_loop()

    registry = get_registry()

    for config in servers:
        if not manager.connect_server(config):
            logger.warning(f"Failed to connect to MCP server: {config.name}")
            continue

        for tool in manager.list_tools(config.name):
            tool_full_name = f"mcp_{config.name}_{tool['name']}"

            handler = _mcp_handler(config.name, tool["name"])

            schema = tool.get("input_schema", {})
            if isinstance(schema, str):
                try:
                    schema = json.loads(schema)
                except Exception:
                    schema = {"type": "object", "properties": {}}

            registry.register(
                name=tool_full_name,
                description=f"[MCP:{config.name}] {tool.get('description', '')}",
                schema=schema,
                handler=handler,
                is_async=True,
            )

            logger.debug(f"Registered MCP tool: {tool_full_name}")


def register_server_tools(server_name: str) -> int:
    """Register tools for a connected server.

    Returns:
        Number of tools registered
    """
    if not MCP_AVAILABLE:
        return 0

    manager = MCPToolManager.get_instance()
    registry = get_registry()
    count = 0

    for tool in manager.list_tools(server_name):
        tool_full_name = f"mcp_{server_name}_{tool['name']}"

        handler = _mcp_handler(server_name, tool["name"])

        schema = tool.get("input_schema", {})
        if isinstance(schema, str):
            try:
                schema = json.loads(schema)
            except Exception:
                schema = {"type": "object", "properties": {}}

        try:
            registry.register(
                name=tool_full_name,
                description=f"[MCP:{server_name}] {tool.get('description', '')}",
                schema=schema,
                handler=handler,
                is_async=True,
            )
            count += 1
        except Exception as e:
            logger.warning(f"Failed to register tool {tool_full_name}: {e}")

    return count


def create_mcp_tool(
    server_name: str,
    tool_name: str,
    description: str = "",
    input_schema: dict | None = None,
) -> tuple[str, dict, callable]:
    """Create an MCP tool tuple for registration.

    Returns:
        (tool_name, schema, handler)
    """
    tool_full_name = f"mcp_{server_name}_{tool_name}"

    async def handler(**kwargs) -> str:
        manager = MCPToolManager.get_instance()
        return await manager.call_tool_async(server_name, tool_name, kwargs)

    schema = input_schema or {"type": "object", "properties": {}}
    if "properties" not in schema:
        schema["properties"] = {}

    return tool_full_name, schema, handler


class MCPTool:
    """MCP tool wrapper for langchain."""

    def __init__(self, server_name: str, tool_name: str, description: str = ""):
        self.server_name = server_name
        self.tool_name = tool_name
        self.description = description
        self.name = f"mcp_{server_name}_{tool_name}"

    async def ainvoke(self, input: dict | str) -> str:
        """Invoke the tool asynchronously."""
        if isinstance(input, str):
            input = {"query": input}
        manager = MCPToolManager.get_instance()
        return await manager.call_tool_async(self.server_name, self.tool_name, input)

    def invoke(self, input: dict | str) -> str:
        """Invoke the tool synchronously."""
        return asyncio.run(self.ainvoke(input))
