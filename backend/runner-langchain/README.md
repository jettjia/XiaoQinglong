# Runner-LangChain

LangChain-based agent runner with hermes-agent style delegation.

## Features

- **Single ReAct Loop**: Based on hermes-agent design philosophy
- **Batch Delegation**: Parallel task delegation up to 3 concurrent sub-agents
- **Depth Limiting**: Max 2 levels (parent -> child -> grandchild)
- **Context Isolation**: Sub-agents run with isolated context
- **Tool Restrictions**: Blocked tools filtered for sub-agents
- **Compatible API**: HTTP API compatible with Go Runner

## Quick Start

```bash
# Install dependencies
cd runner-langchain
uv sync

# Run the server
uv run uvicorn app.main:app --port 18088

# Or run directly
uv run python -m app.main
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/run` | POST | Run agent (compatible with Go Runner) |
| `/agent` | POST | Agent autonomous execution |
| `/health` | GET | Health check |
| `/agents` | GET | List available sub-agents |

## Configuration

The runner accepts JSON requests with:

- `prompt`: System prompt
- `models`: Model configurations (provider, name, api_key, etc.)
- `sub_agents`: Sub-agent configurations
- `options`: Runtime options (max_iterations, temperature, etc.)

## Delegation

Use `delegate_to_agent` tool to delegate tasks:

```json
{
  "tool": "delegate_to_agent",
  "input": {
    "tasks": [
      {"goal": "search AI news", "tools": ["web_search"]},
      {"goal": "read file", "tools": ["file_read"]}
    ],
    "depth": 0
  }
}
```
