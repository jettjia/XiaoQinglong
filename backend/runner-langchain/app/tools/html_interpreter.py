"""HTML interpreter tool for rendering HTML reports.

Based on Go runner's html_interpreter.go design.
Renders HTML content from templates with placeholder replacement.
"""

import json
import logging
import os
import re
import time
from pathlib import Path
from typing import Any, Optional

from app.tools.registry import get_registry
from app.tools.skill_script import get_skill_markers, clear_skill_markers
from app.utils.dir import get_skills_dir

logger = logging.getLogger(__name__)

# Reports directory
REPORTS_DIR = Path.home() / ".xiaoqinglong" / "data" / "reports"


def _extract_title(html: str) -> str:
    """Extract <title> content from HTML."""
    patterns = [
        r"<title>(.*?)</title>",
        r"<TITLE>(.*?)</TITLE>",
    ]
    for pattern in patterns:
        match = re.search(pattern, html, re.DOTALL)
        if match:
            return match.group(1).strip()
    return ""


def _extract_skill_name_from_path(template_path: str) -> str:
    """Extract skill name from template path."""
    parts = template_path.split("/")
    if parts:
        first = parts[0]
        if "-" in first or "_" in first:
            return first
    return ""


def _replace_placeholders(content: str, data: dict[str, Any]) -> str:
    """Replace {{PLACEHOLDER}} placeholders with values."""
    result = content

    def replacer(match):
        key = match.group(1)
        value = data.get(key)
        if value is not None:
            return str(value)
        return match.group(0)

    # Replace all {{KEY}} patterns
    pattern = re.compile(r"\{\{(\w+)\}\}")
    return pattern.sub(replacer, result)


def _save_html_report(output_path: str, content: str, session_id: str) -> str:
    """Save HTML to reports directory. Returns URL path or empty string."""
    reports_dir = REPORTS_DIR

    if session_id:
        reports_dir = reports_dir / session_id

    reports_dir.mkdir(parents=True, exist_ok=True)

    # Clean path to prevent traversal
    output_path = Path(output_path).name

    full_path = reports_dir / output_path

    try:
        full_path.write_text(content, encoding="utf-8")
    except Exception as e:
        logger.warning("[HtmlInterpreter] Failed to save report: %s", e)
        return ""

    if session_id:
        return f"/reports/{session_id}/{output_path}"
    return f"/reports/{output_path}"


def _render_html(
    template_path: Optional[str] = None,
    data: Optional[dict[str, Any]] = None,
    html: Optional[str] = None,
    title: Optional[str] = None,
    output_path: Optional[str] = None,
    session_id: Optional[str] = None,
) -> str:
    """Render HTML content or template.

    Args:
        template_path: Path to HTML template
        data: Placeholder values for template
        html: Direct HTML content
        title: Page title for direct HTML mode
        output_path: Path to save the report
        session_id: Session ID for session-specific storage

    Returns:
        JSON string with result
    """
    data = data or {}
    html_content = ""
    page_title = ""

    if html:
        # Direct HTML mode
        html_content = html
        page_title = title or "Report"
    else:
        # Template mode
        if not template_path:
            return json.dumps({
                "type": "html_report",
                "url": "",
                "title": "",
                "html": "",
                "saved": False,
                "message": "template_path or html is required",
            })

        # Resolve template path
        full_path = template_path
        skills_dir = get_skills_dir()

        if not Path(full_path).is_absolute():
            # Try skills dir
            skill_name = _extract_skill_name_from_path(template_path)
            if skill_name:
                candidate = skills_dir / template_path
                if candidate.exists():
                    full_path = str(candidate)

            if not Path(full_path).exists():
                return json.dumps({
                    "type": "html_report",
                    "url": "",
                    "title": "",
                    "html": "",
                    "saved": False,
                    "message": f"Template file not found: {template_path}",
                })

        # Read template
        try:
            html_content = Path(full_path).read_text(encoding="utf-8")
        except Exception as e:
            return json.dumps({
                "type": "html_report",
                "url": "",
                "title": "",
                "html": "",
                "saved": False,
                "message": f"Error reading template: {e}",
            })

        # Auto-inject skill markers
        markers: dict[str, Any] = {}
        skill_name = _extract_skill_name_from_path(template_path)
        if skill_name:
            stored_markers = get_skill_markers(skill_name)
            if stored_markers:
                markers.update(stored_markers)
                clear_skill_markers(skill_name)

        # Merge with input data (data takes precedence)
        final_data = markers
        final_data.update(data)

        # Replace placeholders
        if final_data:
            html_content = _replace_placeholders(html_content, final_data)

        # Extract title
        page_title = _extract_title(html_content) or "Report"

    # Handle output path
    if not output_path:
        timestamp = time.strftime("%Y%m%d_%H%M%S")
        safe_title = re.sub(r"[^a-zA-Z0-9]+", "_", page_title)[:20]
        output_path = f"report_{safe_title}_{timestamp}.html"

    # Save report
    saved_url = _save_html_report(output_path, html_content, session_id or "")

    return json.dumps({
        "type": "html_report",
        "url": saved_url,
        "title": page_title,
        "html": html_content,
        "saved": saved_url != "",
        "message": "HTML rendered successfully",
    })


def register_tools() -> None:
    """Register html_interpreter tool."""
    registry = get_registry()

    registry.register(
        name="html_interpreter",
        description="Render HTML content as a web report. Supports template-based rendering with {{PLACEHOLDER}} replacement or direct HTML input. Use output_path to save the report.",
        schema={
            "type": "object",
            "properties": {
                "template_path": {
                    "type": "string",
                    "description": "Path to HTML template (relative to skills dir or absolute). Example: csv-data-analysis/templates/report_template.html",
                },
                "data": {
                    "type": "object",
                    "description": "Map of placeholder keys to values for template replacement. Example: {'REPORT_TITLE': 'My Report'}",
                },
                "html": {
                    "type": "string",
                    "description": "Direct HTML content (used instead of template_path)",
                },
                "title": {
                    "type": "string",
                    "description": "Page title for direct HTML mode",
                },
                "output_path": {
                    "type": "string",
                    "description": "Path to save the rendered HTML file (relative to reports dir)",
                },
                "session_id": {
                    "type": "string",
                    "description": "Session ID for session-specific report storage",
                },
            },
            "required": [],
        },
        handler=lambda **kwargs: _render_html(
            template_path=kwargs.get("template_path"),
            data=kwargs.get("data"),
            html=kwargs.get("html"),
            title=kwargs.get("title"),
            output_path=kwargs.get("output_path"),
            session_id=kwargs.get("session_id"),
        ),
    )