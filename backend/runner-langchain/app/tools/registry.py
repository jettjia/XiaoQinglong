"""Tool registry for runner-langchain.

Based on hermes-agent's tool registry design.
"""

import json
import logging
from typing import Any, Callable
from dataclasses import dataclass

from langchain_core.tools import BaseTool, tool

logger = logging.getLogger(__name__)


@dataclass
class ToolInfo:
    """Tool metadata."""

    name: str
    description: str
    schema: dict[str, Any]
    handler: Callable
    is_async: bool = False


class ToolRegistry:
    """Central registry for all tools."""

    _instance: "ToolRegistry | None" = None

    def __init__(self):
        self._tools: dict[str, ToolInfo] = {}

    @classmethod
    def get_instance(cls) -> "ToolRegistry":
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance

    def register(
        self,
        name: str,
        description: str,
        schema: dict[str, Any],
        handler: Callable,
        is_async: bool = False,
    ) -> None:
        """Register a tool."""
        self._tools[name] = ToolInfo(
            name=name,
            description=description,
            schema=schema,
            handler=handler,
            is_async=is_async,
        )
        logger.debug(f"Registered tool: {name}")

    def get(self, name: str) -> ToolInfo | None:
        """Get a tool by name."""
        return self._tools.get(name)

    def get_langchain_tools(self) -> list[BaseTool]:
        """Get tools as langchain BaseTool objects."""
        tools = []
        for tool_info in self._tools.values():
            lc_tool = self._create_langchain_tool(tool_info)
            if lc_tool:
                tools.append(lc_tool)
        return tools

    def _create_langchain_tool(self, tool_info: ToolInfo) -> BaseTool | None:
        """Create a langchain BaseTool from ToolInfo."""
        try:
            from pydantic import create_model

            # Create args_schema from tool schema
            schema_props = tool_info.schema.get("properties", {})
            required = tool_info.schema.get("required", [])

            # Build field definitions for create_model
            field_defs = {}
            for prop_name, prop_info in schema_props.items():
                # Extract default if present
                default = prop_info.pop("default", ...)
                # Determine type
                prop_type = prop_info.get("type", "string")
                if prop_type == "integer":
                    field_type = int
                elif prop_type == "boolean":
                    field_type = bool
                elif prop_type == "array":
                    field_type = list
                else:
                    field_type = str
                # Add field (default determines required vs optional)
                if default is ...:
                    field_defs[prop_name] = (field_type, ...)
                else:
                    field_defs[prop_name] = (field_type, default)

            # Create the args_schema dynamically
            model_name = f"{tool_info.name.title().replace('_', '')}Args"
            args_schema = create_model(model_name, **field_defs)

            # Create wrapper for handler that accepts **kwargs
            handler = tool_info.handler

            @tool(tool_info.name, description=tool_info.description, args_schema=args_schema)
            def tool_wrapper(**kwargs) -> str:
                return handler(**kwargs)

            return tool_wrapper
        except Exception as e:
            logger.error(f"Failed to create langchain tool {tool_info.name}: {e}")
            return None

    def list_tools(self) -> list[ToolInfo]:
        """List all registered tools."""
        return list(self._tools.values())

    def invoke(self, name: str, arguments: dict[str, Any]) -> str:
        """Invoke a tool by name."""
        tool = self._tools.get(name)
        if not tool:
            return json.dumps({"error": f"Tool not found: {name}"})

        try:
            result = tool.handler(**arguments)
            if isinstance(result, str):
                return result
            return json.dumps(result)
        except Exception as e:
            logger.error(f"Tool {name} failed: {e}")
            return json.dumps({"error": str(e)})


def get_registry() -> ToolRegistry:
    """Get the tool registry instance."""
    return ToolRegistry.get_instance()
