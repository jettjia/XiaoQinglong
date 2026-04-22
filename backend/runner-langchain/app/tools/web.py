"""Web search and fetch tools."""

import json
from typing import Any

try:
    import httpx
except ImportError:
    httpx = None

from app.tools.registry import get_registry


def _web_search(query: str, num_results: int = 5) -> str:
    """Search the web for information.

    Args:
        query: Search query
        num_results: Number of results to return

    Returns:
        JSON string with search results
    """
    if httpx is None:
        return json.dumps({"error": "httpx not installed"})

    try:
        # Using DuckDuckGo HTML search (no API key required)
        url = "https://duckduckgo.com/html/"
        params = {"q": query}

        response = httpx.get(url, params=params, timeout=10.0)
        response.raise_for_status()

        # Simple HTML parsing to extract results
        content = response.text

        # Extract titles and links from search results
        results = []
        import re

        # Find result snippets
        pattern = r'<a class="result__snippet"[^>]*href="([^"]*)"[^>]*>([^<]*)</a>'
        matches = re.findall(pattern, content)

        for url, snippet in matches[:num_results]:
            results.append({
                "url": url,
                "snippet": snippet.strip(),
            })

        if not results:
            # Alternative pattern for regular results
            pattern = r'<a class="result__a" href="([^"]*)"[^>]*>([^<]*)</a>'
            matches = re.findall(pattern, content)
            for url, title in matches[:num_results]:
                results.append({
                    "url": url,
                    "title": title.strip(),
                })

        return json.dumps({
            "query": query,
            "count": len(results),
            "results": results,
        })
    except Exception as e:
        return json.dumps({"error": str(e)})


def _web_fetch(url: str, max_chars: int = 5000) -> str:
    """Fetch content from a URL.

    Args:
        url: URL to fetch
        max_chars: Maximum number of characters to return

    Returns:
        JSON string with fetched content
    """
    if httpx is None:
        return json.dumps({"error": "httpx not installed"})

    try:
        response = httpx.get(url, timeout=10.0, follow_redirects=True)
        response.raise_for_status()

        content = response.text

        # Truncate if needed
        if len(content) > max_chars:
            content = content[:max_chars] + f"\n... [truncated, original {len(content)} chars]"

        return json.dumps({
            "url": str(response.url),
            "status_code": response.status_code,
            "content_type": response.headers.get("content-type", ""),
            "content": content,
        })
    except Exception as e:
        return json.dumps({"error": str(e)})


def register_tools() -> None:
    """Register web tools."""
    registry = get_registry()

    registry.register(
        name="web_search",
        description="Search the web for information",
        schema={
            "type": "object",
            "properties": {
                "query": {"type": "string", "description": "Search query"},
                "num_results": {"type": "integer", "description": "Number of results", "default": 5},
            },
            "required": ["query"],
        },
        handler=_web_search,
    )

    registry.register(
        name="web_fetch",
        description="Fetch content from a URL",
        schema={
            "type": "object",
            "properties": {
                "url": {"type": "string", "description": "URL to fetch"},
                "max_chars": {"type": "integer", "description": "Max characters", "default": 5000},
            },
            "required": ["url"],
        },
        handler=_web_fetch,
    )
