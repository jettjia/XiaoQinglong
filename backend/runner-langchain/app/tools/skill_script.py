"""Execute skill script files tool.

Based on Go runner's execute_skill_script_file.go design.
Executes Python/Bash scripts from skills directory.
"""

import contextlib
import json
import logging
import os
import re
import subprocess
import sys
import tempfile
import threading
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Optional

from app.tools.registry import get_registry
from app.utils.dir import get_skills_dir

logger = logging.getLogger(__name__)

# Global marker storage for sharing data between tools
_marker_storage: dict[str, dict[str, str]] = {}
_marker_lock = threading.Lock()


def set_skill_markers(skill_name: str, markers: dict[str, str]) -> None:
    """Store marker data for a skill."""
    with _marker_lock:
        _marker_storage[skill_name] = markers


def get_skill_markers(skill_name: str) -> Optional[dict[str, str]]:
    """Retrieve marker data for a skill."""
    with _marker_lock:
        return _marker_storage.get(skill_name)


def clear_skill_markers(skill_name: str) -> None:
    """Remove marker data for a skill."""
    with _marker_lock:
        _marker_storage.pop(skill_name, None)


@dataclass
class ScriptChunk:
    """A chunk of script output."""
    output_type: str  # text, image, code, data
    content: str
    key: Optional[str] = None


def _extract_markers(output: str) -> dict[str, str]:
    """Extract ###KEY_START###...###KEY_END### blocks from output."""
    markers = {}

    # Find all KEY names from START markers
    start_pattern = re.compile(r"###(\w+)_START###")
    for match in start_pattern.finditer(output):
        key = match.group(1)
        start_pos = match.end()

        end_marker = f"###{key}_END###"
        end_pos = output.find(end_marker, start_pos)

        if end_pos >= 0:
            # Content is between START marker end and END marker start
            content_start = start_pos
            content_end = end_pos
            if content_start < content_end:
                markers[key] = output[content_start:content_end]

    return markers


def _execute_script(
    skill_name: str,
    script_file_name: str,
    args: Optional[dict[str, Any]] = None,
    output_dir: Optional[str] = None,
) -> str:
    """Execute a script from a skill's scripts directory.

    Args:
        skill_name: Skill name (e.g., "csv-data-analysis")
        script_file_name: Script filename (e.g., "csv_analyzer.py")
        args: Arguments to pass to the script
        output_dir: Output directory for execution

    Returns:
        JSON string with chunks
    """
    args = args or {}

    # Translate sandbox mount paths
    if "input_file" in args and isinstance(args["input_file"], str):
        input_file = args["input_file"]
        if input_file.startswith("/mnt/uploads/"):
            uploads_dir = os.environ.get("XQL_UPLOADS_DIR", "")
            if not uploads_dir:
                uploads_dir = Path.home() / ".xiaoqinglong" / "data" / "uploads"
            args["input_file"] = input_file.replace("/mnt/uploads/", str(uploads_dir) + "/")

    # Resolve skills directory
    skills_dir = get_skills_dir()

    # Build script path
    script_file_name = script_file_name.lstrip("scripts/").lstrip("scripts\\")
    script_path = skills_dir / skill_name / "scripts" / script_file_name

    if not script_path.exists():
        return json.dumps({
            "chunks": [
                {"output_type": "text", "content": f"Script file not found: {script_path}"}
            ]
        })

    # Determine working directory
    work_dir = output_dir or str(script_path.parent)
    Path(work_dir).mkdir(parents=True, exist_ok=True)

    # Read script content
    try:
        script_content = script_path.read_text(encoding="utf-8")
    except Exception as e:
        return json.dumps({
            "chunks": [
                {"output_type": "text", "content": f"Error reading script: {e}"}
            ]
        })

    # Determine script type and wrap code
    ext = script_path.suffix.lower()
    chunks = []

    if ext == ".py":
        # Wrap Python script
        args_json = json.dumps(args)
        exec_code = f"""import sys
import json

sys.argv = ["script", {args_json}]
__name__ = "__main__"

{script_content}"""
    else:
        # Bash script - execute directly
        exec_code = script_content

    # Execute script
    stdout = ""
    stderr = ""
    exit_code = 0

    try:
        if ext == ".py":
            # Write to temp file
            with tempfile.NamedTemporaryFile(
                mode="w",
                suffix="_skill_run_.py",
                dir=script_path.parent,
                delete=False,
                encoding="utf-8",
            ) as tmp_file:
                tmp_file.write(exec_code)
                tmp_path = tmp_file.name

            try:
                result = subprocess.run(
                    [sys.executable, tmp_path],
                    capture_output=True,
                    text=True,
                    timeout=120,
                    cwd=work_dir,
                )
                stdout = result.stdout
                stderr = result.stderr
                exit_code = result.returncode
            finally:
                with contextlib.suppress(OSError):
                    os.unlink(tmp_path)
        else:
            # Bash script
            result = subprocess.run(
                ["bash", "-c", exec_code],
                capture_output=True,
                text=True,
                timeout=120,
                cwd=work_dir,
            )
            stdout = result.stdout
            stderr = result.stderr
            exit_code = result.returncode

    except subprocess.TimeoutExpired:
        return json.dumps({
            "chunks": [
                {"output_type": "text", "content": "Script execution timed out (120s)"}
            ]
        })
    except Exception as e:
        return json.dumps({
            "chunks": [
                {"output_type": "text", "content": f"Execution error: {e}"}
            ]
        })

    # Parse output
    output_text = stdout.strip()

    if output_text:
        # Try JSON parsing first
        try:
            parsed = json.loads(output_text)
            if isinstance(parsed, dict) and "chunks" in parsed:
                chunks = parsed["chunks"]
            else:
                chunks = [{"output_type": "text", "content": output_text}]
        except json.JSONDecodeError:
            # Non-JSON output - extract markers and clean
            markers = _extract_markers(output_text)
            clean_content = output_text

            # Remove marker blocks from content
            for key in markers:
                pattern = rf"###{key}_START###.*?###{key}_END###"
                clean_content = re.sub(pattern, "", clean_content, flags=re.DOTALL)

            clean_content = clean_content.strip()

            # Add markers as data chunks
            for key, value in markers.items():
                if value:
                    chunks.append({
                        "output_type": "data",
                        "content": value,
                        "key": key,
                    })

            if clean_content:
                chunks.append({"output_type": "text", "content": clean_content})

    # Store markers for html_interpreter
    if skill_name and chunks:
        all_markers: dict[str, str] = {}
        for chunk in chunks:
            if chunk.get("content"):
                chunk_markers = _extract_markers(chunk["content"])
                all_markers.update(chunk_markers)

        if all_markers:
            set_skill_markers(skill_name, all_markers)

    # Add stderr if present
    if stderr and exit_code != 0:
        chunks.append({"output_type": "text", "content": f"[ERROR] {stderr}"})

    if exit_code != 0 and not any("error" in c.get("content", "").lower() for c in chunks):
        chunks.append({"output_type": "text", "content": f"Exit code: {exit_code}"})

    if not chunks:
        chunks.append({"output_type": "text", "content": "Script executed successfully (no output)"})

    return json.dumps({"chunks": chunks})


def register_tools() -> None:
    """Register execute_skill_script_file tool."""
    registry = get_registry()

    registry.register(
        name="execute_skill_script_file",
        description="Execute a Python script file from a skill's scripts directory. The script receives args as JSON via sys.argv[1]. Returns JSON with chunks.",
        schema={
            "type": "object",
            "properties": {
                "skill_name": {
                    "type": "string",
                    "description": "The name of the skill (e.g., 'csv-data-analysis')",
                },
                "script_file_name": {
                    "type": "string",
                    "description": "The script filename in the skill's scripts directory (e.g., 'csv_analyzer.py')",
                },
                "args": {
                    "type": "object",
                    "description": "Arguments to pass to the script as a JSON object",
                },
                "output_dir": {
                    "type": "string",
                    "description": "Output directory for script execution (optional)",
                },
            },
            "required": ["skill_name", "script_file_name"],
        },
        handler=lambda **kwargs: _execute_script(
            skill_name=kwargs.get("skill_name", ""),
            script_file_name=kwargs.get("script_file_name", ""),
            args=kwargs.get("args"),
            output_dir=kwargs.get("output_dir"),
        ),
    )