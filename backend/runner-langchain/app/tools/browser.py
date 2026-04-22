"""Browser automation tool using Playwright.

Supports:
- Page navigation
- Screenshot capture
- Accessibility tree snapshot
- Element clicking
- Content extraction

Requires: playwright installed and browsers downloaded
    pip install playwright
    playwright install chromium
"""

import asyncio
import json
import logging
from typing import Optional

from app.tools.registry import get_registry

logger = logging.getLogger(__name__)

PLAYWRIGHT_AVAILABLE = False
try:
    from playwright.async_api import async_playwright, Browser, Page, BrowserContext
    PLAYWRIGHT_AVAILABLE = True
except ImportError:
    logger.warning("Playwright not installed. Run: pip install playwright && playwright install chromium")


class BrowserManager:
    """Manages browser instances for automation.

    Based on hermes-agent's browser_tool.py design.
    """

    _instance: Optional["BrowserManager"] = None

    def __init__(self):
        self._playwright = None
        self._browser: Optional[Browser] = None
        self._contexts: dict[str, BrowserContext] = {}
        self._pages: dict[str, Page] = {}

    @classmethod
    def get_instance(cls) -> "BrowserManager":
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance

    async def _ensure_browser(self) -> Browser:
        """Ensure browser is initialized."""
        if self._browser is None or not self._browser.is_connected():
            self._playwright = await async_playwright().start()
            self._browser = await self._playwright.chromium.launch(headless=True)
        return self._browser

    async def _get_page(self, task_id: str) -> tuple[BrowserContext, Page]:
        """Get or create a page for the given task_id."""
        await self._ensure_browser()

        if task_id not in self._contexts:
            context = await self._browser.new_context(
                viewport={"width": 1280, "height": 720},
                user_agent="Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36"
            )
            page = await context.new_page()
            self._contexts[task_id] = context
            self._pages[task_id] = page
        else:
            page = self._pages[task_id]

        return self._contexts[task_id], page

    async def navigate(self, url: str, task_id: str = "default") -> str:
        """Navigate to a URL.

        Args:
            url: Target URL
            task_id: Session identifier

        Returns:
            JSON result with status and initial snapshot
        """
        if not PLAYWRIGHT_AVAILABLE:
            return json.dumps({"error": "Playwright not installed"})

        try:
            _, page = await self._get_page(task_id)
            response = await page.goto(url, wait_until="domcontentloaded", timeout=30000)

            # Get initial snapshot
            snapshot = await self._get_snapshot(page)

            return json.dumps({
                "success": True,
                "url": page.url,
                "title": await page.title(),
                "status_code": response.status if response else None,
                "snapshot": snapshot,
            })
        except Exception as e:
            logger.error(f"Browser navigate failed: {e}")
            return json.dumps({"error": str(e)})

    async def snapshot(self, task_id: str = "default", max_length: int = 5000) -> str:
        """Get current page snapshot.

        Args:
            task_id: Session identifier
            max_length: Maximum content length

        Returns:
            JSON result with page content
        """
        if not PLAYWRIGHT_AVAILABLE:
            return json.dumps({"error": "Playwright not installed"})

        try:
            if task_id not in self._pages:
                return json.dumps({"error": f"No active page for task: {task_id}"})

            page = self._pages[task_id]
            snapshot = await self._get_snapshot(page, max_length)

            return json.dumps({
                "success": True,
                "url": page.url,
                "title": await page.title(),
                "snapshot": snapshot,
            })
        except Exception as e:
            logger.error(f"Browser snapshot failed: {e}")
            return json.dumps({"error": str(e)})

    async def click(self, selector: str, task_id: str = "default") -> str:
        """Click an element by selector.

        Args:
            selector: Element selector (@e1, text, or CSS)
            task_id: Session identifier

        Returns:
            JSON result
        """
        if not PLAYWRIGHT_AVAILABLE:
            return json.dumps({"error": "Playwright not installed"})

        try:
            if task_id not in self._pages:
                return json.dumps({"error": f"No active page for task: {task_id}"})

            page = self._pages[task_id]

            # Handle @e1 style references
            if selector.startswith("@e"):
                # Try to find by index
                elements = await page.query_selector_all("[data-ref]")
                idx = int(selector[2:]) - 1
                if 0 <= idx < len(elements):
                    await elements[idx].click()
            else:
                # Try CSS selector or text
                try:
                    await page.click(selector, timeout=5000)
                except Exception:
                    # Try by text content
                    element = await page.get_by_text(selector, exact=False).first
                    if element:
                        await element.click()
                    else:
                        return json.dumps({"error": f"Element not found: {selector}"})

            # Get updated snapshot
            await page.wait_for_load_state("domcontentloaded")
            snapshot = await self._get_snapshot(page)

            return json.dumps({
                "success": True,
                "snapshot": snapshot,
            })
        except Exception as e:
            logger.error(f"Browser click failed: {e}")
            return json.dumps({"error": str(e)})

    async def type_(self, selector: str, text: str, task_id: str = "default") -> str:
        """Type text into an element.

        Args:
            selector: Element selector
            text: Text to type
            task_id: Session identifier

        Returns:
            JSON result
        """
        if not PLAYWRIGHT_AVAILABLE:
            return json.dumps({"error": "Playwright not installed"})

        try:
            if task_id not in self._pages:
                return json.dumps({"error": f"No active page for task: {task_id}"})

            page = self._pages[task_id]
            await page.fill(selector, text)

            return json.dumps({
                "success": True,
                "filled": selector,
            })
        except Exception as e:
            logger.error(f"Browser type failed: {e}")
            return json.dumps({"error": str(e)})

    async def screenshot(self, task_id: str = "default") -> str:
        """Take a screenshot of the current page.

        Args:
            task_id: Session identifier

        Returns:
            JSON result with base64 screenshot
        """
        if not PLAYWRIGHT_AVAILABLE:
            return json.dumps({"error": "Playwright not installed"})

        try:
            if task_id not in self._pages:
                return json.dumps({"error": f"No active page for task: {task_id}"})

            page = self._pages[task_id]
            screenshot_bytes = await page.screenshot(full_page=True)
            import base64
            screenshot_b64 = base64.b64encode(screenshot_bytes).decode()

            return json.dumps({
                "success": True,
                "screenshot": screenshot_b64,
            })
        except Exception as e:
            logger.error(f"Browser screenshot failed: {e}")
            return json.dumps({"error": str(e)})

    async def _get_snapshot(self, page: Page, max_length: int = 5000) -> str:
        """Get accessibility tree snapshot of the page."""
        try:
            # Get accessible elements
            content = await page.content()
            title = await page.title()
            url = page.url

            # Extract visible text content
            text_content = await page.evaluate("""
                () => {
                    const walker = document.createTreeWalker(
                        document.body,
                        NodeFilter.SHOW_TEXT,
                        null,
                        false
                    );
                    const texts = [];
                    let node;
                    while (node = walker.nextNode()) {
                        const text = node.textContent.trim();
                        if (text.length > 0) {
                            texts.push(text);
                        }
                    }
                    return texts.join(' ');
                }
            """)

            # Build snapshot with accessible elements
            elements = await page.query_selector_all("a, button, input, [role], h1, h2, h3, h4, h5, h6")
            accessible = []
            for i, elem in enumerate(elements[:50], 1):  # Limit to 50 elements
                tag = await elem.evaluate("el => el.tagName.toLowerCase()")
                text = (await elem.text_content() or "").strip()[:100]
                role = await elem.evaluate("el => el.getAttribute('role') or ''")
                ref = f"@e{i}"

                if text:
                    accessible.append(f"{ref} <{tag}>{text}</{tag}>" if role == "button" or tag == "button" else f"{ref} {tag}: {text}")

            snapshot = f"URL: {url}\nTitle: {title}\n\nElements:\n" + "\n".join(accessible[:20])

            if len(snapshot) > max_length:
                snapshot = snapshot[:max_length] + "...[truncated]"

            return snapshot

        except Exception as e:
            logger.error(f"Get snapshot failed: {e}")
            return f"Error getting snapshot: {str(e)}"

    async def close(self, task_id: str = "default") -> None:
        """Close a browser context."""
        if task_id in self._contexts:
            await self._contexts[task_id].close()
            del self._contexts[task_id]
            if task_id in self._pages:
                del self._pages[task_id]

    async def close_all(self) -> None:
        """Close all browser contexts and browser."""
        for task_id in list(self._contexts.keys()):
            await self.close(task_id)

        if self._browser:
            await self._browser.close()
            self._browser = None


# Synchronous wrappers for tool registry

def _sync_navigate(url: str, task_id: str = "default") -> str:
    """Sync wrapper for navigate."""
    manager = BrowserManager.get_instance()
    return asyncio.run(manager.navigate(url, task_id))


def _sync_snapshot(task_id: str = "default", max_length: int = 5000) -> str:
    """Sync wrapper for snapshot."""
    manager = BrowserManager.get_instance()
    return asyncio.run(manager.snapshot(task_id, max_length))


def _sync_click(selector: str, task_id: str = "default") -> str:
    """Sync wrapper for click."""
    manager = BrowserManager.get_instance()
    return asyncio.run(manager.click(selector, task_id))


def _sync_type(selector: str, text: str, task_id: str = "default") -> str:
    """Sync wrapper for type."""
    manager = BrowserManager.get_instance()
    return asyncio.run(manager.type_(selector, text, task_id))


def _sync_screenshot(task_id: str = "default") -> str:
    """Sync wrapper for screenshot."""
    manager = BrowserManager.get_instance()
    return asyncio.run(manager.screenshot(task_id))


def _sync_close(task_id: str = "default") -> str:
    """Sync wrapper for close."""
    manager = BrowserManager.get_instance()
    asyncio.run(manager.close(task_id))
    return json.dumps({"success": True})


def register_tools() -> None:
    """Register browser automation tools."""
    if not PLAYWRIGHT_AVAILABLE:
        logger.warning("Playwright not available, browser tools not registered")
        return

    registry = get_registry()

    registry.register(
        name="browser_navigate",
        description="Navigate to a URL in the browser",
        schema={
            "type": "object",
            "properties": {
                "url": {"type": "string", "description": "Target URL"},
                "task_id": {"type": "string", "description": "Session ID", "default": "default"},
            },
            "required": ["url"],
        },
        handler=_sync_navigate,
    )

    registry.register(
        name="browser_snapshot",
        description="Get current page content snapshot",
        schema={
            "type": "object",
            "properties": {
                "task_id": {"type": "string", "description": "Session ID", "default": "default"},
                "max_length": {"type": "integer", "description": "Max content length", "default": 5000},
            },
        },
        handler=_sync_snapshot,
    )

    registry.register(
        name="browser_click",
        description="Click an element by selector",
        schema={
            "type": "object",
            "properties": {
                "selector": {"type": "string", "description": "Element selector (@e1, CSS, or text)"},
                "task_id": {"type": "string", "description": "Session ID", "default": "default"},
            },
            "required": ["selector"],
        },
        handler=_sync_click,
    )

    registry.register(
        name="browser_type",
        description="Type text into an element",
        schema={
            "type": "object",
            "properties": {
                "selector": {"type": "string", "description": "Element selector"},
                "text": {"type": "string", "description": "Text to type"},
                "task_id": {"type": "string", "description": "Session ID", "default": "default"},
            },
            "required": ["selector", "text"],
        },
        handler=_sync_type,
    )

    registry.register(
        name="browser_screenshot",
        description="Take a screenshot of the current page",
        schema={
            "type": "object",
            "properties": {
                "task_id": {"type": "string", "description": "Session ID", "default": "default"},
            },
        },
        handler=_sync_screenshot,
    )

    registry.register(
        name="browser_close",
        description="Close browser session",
        schema={
            "type": "object",
            "properties": {
                "task_id": {"type": "string", "description": "Session ID", "default": "default"},
            },
        },
        handler=_sync_close,
    )
