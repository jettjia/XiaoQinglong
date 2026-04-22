"""Skills tool for listing and viewing skill documents.

Based on hermes-agent's skills_tool.py design with progressive disclosure:
- Metadata (name ≤64 chars, description ≤1024 chars) - shown in skills_list
- Full Instructions - loaded via skill_view when needed
- Linked Files (references, templates) - loaded on demand

Directory Structure:
    ~/.xiaoqinglong/skills/
    ├── my-skill/
    │   ├── SKILL.md           # Main instructions (required)
    │   ├── references/        # Supporting documentation
    │   │   └── api.md
    │   ├── templates/        # Templates for output
    │   │   └── template.md
    │   └── assets/           # Supplementary files
    └── category/
        └── another-skill/
            └── SKILL.md

SKILL.md Format (YAML Frontmatter):
    ---
    name: skill-name
    description: Brief description
    version: 1.0.0
    platforms: [macos, linux, windows]
    tags: [tag1, tag2]
    related_skills: [other-skill]
    ---
"""

import json
import logging
import os
import sys
from pathlib import Path
from typing import Any, Optional

import frontmatter

from app.tools.registry import get_registry
from app.utils.dir import get_skills_dir, init as dir_init
from app.skills.manager import get_skill_manager

logger = logging.getLogger(__name__)

# Initialize directories on module load
dir_init()

# Default skills directories (follows xqldir design)
_SKILLS_DIR = get_skills_dir()

# Additional skills directories (user-configurable)
_EXTERNAL_SKILLS_DIRS: list[Path] = []

# Limits for progressive disclosure
MAX_NAME_LENGTH = 64
MAX_DESCRIPTION_LENGTH = 1024

# Platform mapping
_PLATFORM_MAP = {
    "macos": "darwin",
    "linux": "linux",
    "windows": "win32",
}

_EXCLUDED_SKILL_DIRS = frozenset((".git", ".github", ".hub", ".venv", "node_modules"))


def _get_skills_dirs() -> list[Path]:
    """Get list of skills directories to search.

    Follows xqldir design: uses get_skills_dir() as primary,
    plus any external dirs configured.
    """
    dirs = [_SKILLS_DIR]
    for d in _EXTERNAL_SKILLS_DIRS:
        if d.exists():
            dirs.append(d)
    return [d for d in dirs if d.exists()]


def add_external_skills_dir(path: str) -> None:
    """Add an external skills directory to search.

    Args:
        path: Path to external skills directory
    """
    expanded = Path(os.path.expanduser(path))
    if expanded.exists() and expanded not in _EXTERNAL_SKILLS_DIRS:
        _EXTERNAL_SKILLS_DIRS.append(expanded)


def _skill_matches_platform(frontmatter: dict) -> bool:
    """Check if a skill is compatible with the current OS platform."""
    platforms = frontmatter.get("platforms", [])
    if not platforms:
        return True  # No restriction = all platforms

    current_platform = sys.platform
    for platform in platforms:
        mapped = _PLATFORM_MAP.get(platform, platform)
        if mapped == current_platform:
            return True
    return False


def _parse_frontmatter(content: str) -> tuple[dict, str]:
    """Parse YAML frontmatter from markdown content."""
    try:
        parsed = frontmatter.parse(content)
        if isinstance(parsed, tuple) and len(parsed) == 2:
            # frontmatter.parse returns (metadata_dict, content)
            return parsed[0], parsed[1]
        # Older frontmatter versions return an object
        return parsed.metadata, parsed.content
    except Exception:
        return {}, content


def _get_category_from_path(skill_path: Path) -> Optional[str]:
    """Extract category from skill path based on directory structure."""
    skills_dirs = _get_skills_dirs()

    for skills_dir in skills_dirs:
        try:
            rel_path = skill_path.relative_to(skills_dir)
            parts = rel_path.parts
            if len(parts) >= 3:  # category/skill-name/SKILL.md
                return parts[0]
        except ValueError:
            continue
    return None


def _find_all_skills() -> list[dict]:
    """Recursively find all skills in configured directories."""
    skills = []
    seen_names = set()

    for skills_dir in _get_skills_dirs():
        if not skills_dir.exists():
            continue

        for skill_md in skills_dir.rglob("SKILL.md"):
            # Skip excluded directories
            if any(excluded in skill_md.parts for excluded in _EXCLUDED_SKILL_DIRS):
                continue

            skill_dir = skill_md.parent

            try:
                content = skill_md.read_text(encoding="utf-8")[:4000]
                frontmatter_data, body = _parse_frontmatter(content)

                if not _skill_matches_platform(frontmatter_data):
                    continue

                name = frontmatter_data.get("name", skill_dir.name)[:MAX_NAME_LENGTH]
                if name in seen_names:
                    continue

                description = frontmatter_data.get("description", "")
                if not description:
                    # Use first non-header line as description
                    for line in body.strip().split("\n"):
                        line = line.strip()
                        if line and not line.startswith("#"):
                            description = line
                            break

                if len(description) > MAX_DESCRIPTION_LENGTH:
                    description = description[:MAX_DESCRIPTION_LENGTH - 3] + "..."

                category = _get_category_from_path(skill_md)

                seen_names.add(name)
                skills.append({
                    "name": name,
                    "description": description,
                    "category": category,
                })

            except (UnicodeDecodeError, PermissionError) as e:
                logger.debug("Failed to read skill file %s: %s", skill_md, e)
                continue
            except Exception as e:
                logger.debug("Skipping skill at %s: failed to parse: %s", skill_md, e)
                continue

    return skills


def _skills_list(category: str = None) -> str:
    """List all available skills (progressive disclosure tier 1).

    Returns only name + description to minimize token usage.
    """
    try:
        skills_dirs = _get_skills_dirs()

        if not skills_dirs:
            return json.dumps({
                "success": True,
                "skills": [],
                "categories": [],
                "message": "No skills directory configured.",
            }, ensure_ascii=False)

        # Find all skills
        all_skills = _find_all_skills()

        if not all_skills:
            return json.dumps({
                "success": True,
                "skills": [],
                "categories": [],
                "message": "No skills found in skills/ directory.",
            }, ensure_ascii=False)

        # Filter by category if specified
        if category:
            all_skills = [s for s in all_skills if s.get("category") == category]

        # Sort by category then name
        all_skills.sort(key=lambda s: (s.get("category") or "", s["name"]))

        # Extract unique categories
        categories = sorted(
            set(s.get("category") for s in all_skills if s.get("category"))
        )

        return json.dumps({
            "success": True,
            "skills": all_skills,
            "categories": categories,
            "count": len(all_skills),
            "hint": "Use skill_view(name) to see full content",
        }, ensure_ascii=False)

    except Exception as e:
        logger.error("skills_list failed: %s", e)
        return json.dumps({"success": False, "error": str(e)}, ensure_ascii=False)


def _skill_view(name: str, file_path: str = None) -> str:
    """View the content of a skill or a specific file within a skill directory.

    Args:
        name: Name or path of the skill
        file_path: Optional path to a specific file within the skill

    Returns:
        JSON string with skill content
    """
    try:
        skills_dirs = _get_skills_dirs()

        if not skills_dirs:
            return json.dumps({
                "success": False,
                "error": "No skills directory configured.",
            }, ensure_ascii=False)

        skill_md = None
        skill_dir = None

        # Search all dirs: local first, then external (first match wins)
        for search_dir in skills_dirs:
            # Try direct path first (e.g., "mlops/axolotl")
            direct_path = search_dir / name
            if direct_path.is_dir() and (direct_path / "SKILL.md").exists():
                skill_dir = direct_path
                skill_md = direct_path / "SKILL.md"
                break
            elif direct_path.with_suffix(".md").exists():
                skill_md = direct_path.with_suffix(".md")
                break

        # Search by directory name across all dirs
        if not skill_md:
            for search_dir in skills_dirs:
                for found_skill_md in search_dir.rglob("SKILL.md"):
                    if found_skill_md.parent.name == name:
                        skill_dir = found_skill_md.parent
                        skill_md = found_skill_md
                        break
                if skill_md:
                    break

        # Legacy: flat .md files
        if not skill_md:
            for search_dir in skills_dirs:
                for found_md in search_dir.rglob(f"{name}.md"):
                    if found_md.name != "SKILL.md":
                        skill_md = found_md
                        break
                if skill_md:
                    break

        if not skill_md or not skill_md.exists():
            available = [s["name"] for s in _find_all_skills()[:20]]
            return json.dumps({
                "success": False,
                "error": f"Skill '{name}' not found.",
                "available_skills": available,
            }, ensure_ascii=False)

        # Read the content
        try:
            content = skill_md.read_text(encoding="utf-8")
        except Exception as e:
            return json.dumps({
                "success": False,
                "error": f"Failed to read skill '{name}': {e}",
            }, ensure_ascii=False)

        frontmatter_data, body = _parse_frontmatter(content)

        if not _skill_matches_platform(frontmatter_data):
            return json.dumps({
                "success": False,
                "error": f"Skill '{name}' is not supported on this platform.",
            }, ensure_ascii=False)

        # If a specific file path is requested, read that instead
        if file_path and skill_dir:
            # Prevent path traversal
            normalized_path = Path(file_path)
            if ".." in normalized_path.parts:
                return json.dumps({
                    "success": False,
                    "error": "Path traversal ('..') is not allowed.",
                }, ensure_ascii=False)

            target_file = skill_dir / file_path

            # Verify path is within skill directory
            try:
                resolved = target_file.resolve()
                skill_dir_resolved = skill_dir.resolve()
                if not resolved.is_relative_to(skill_dir_resolved):
                    return json.dumps({
                        "success": False,
                        "error": "Path escapes skill directory boundary.",
                    }, ensure_ascii=False)
            except (OSError, ValueError):
                return json.dumps({
                    "success": False,
                    "error": f"Invalid file path: '{file_path}'",
                }, ensure_ascii=False)

            if not target_file.exists():
                # List available files
                available_files = {"references": [], "templates": [], "assets": [], "scripts": [], "other": []}

                for f in skill_dir.rglob("*"):
                    if f.is_file() and f.name != "SKILL.md":
                        rel = str(f.relative_to(skill_dir))
                        if rel.startswith("references/"):
                            available_files["references"].append(rel)
                        elif rel.startswith("templates/"):
                            available_files["templates"].append(rel)
                        elif rel.startswith("assets/"):
                            available_files["assets"].append(rel)
                        elif rel.startswith("scripts/"):
                            available_files["scripts"].append(rel)
                        else:
                            available_files["other"].append(rel)

                available_files = {k: v for k, v in available_files.items() if v}

                return json.dumps({
                    "success": False,
                    "error": f"File '{file_path}' not found in skill '{name}'.",
                    "available_files": available_files,
                }, ensure_ascii=False)

            # Read the file
            try:
                file_content = target_file.read_text(encoding="utf-8")
            except UnicodeDecodeError:
                return json.dumps({
                    "success": True,
                    "name": name,
                    "file": file_path,
                    "content": f"[Binary file: {target_file.name}, size: {target_file.stat().st_size} bytes]",
                    "is_binary": True,
                }, ensure_ascii=False)

            return json.dumps({
                "success": True,
                "name": name,
                "file": file_path,
                "content": file_content,
                "file_type": target_file.suffix,
            }, ensure_ascii=False)

        # Get linked files info
        reference_files = []
        template_files = []
        asset_files = []
        script_files = []

        if skill_dir:
            references_dir = skill_dir / "references"
            if references_dir.exists():
                reference_files = [str(f.relative_to(skill_dir)) for f in references_dir.glob("*.md")]

            templates_dir = skill_dir / "templates"
            if templates_dir.exists():
                for ext in ["*.md", "*.yaml", "*.yml", "*.json", "*.sh"]:
                    template_files.extend([str(f.relative_to(skill_dir)) for f in templates_dir.rglob(ext)])

            assets_dir = skill_dir / "assets"
            if assets_dir.exists():
                asset_files = [str(f.relative_to(skill_dir)) for f in assets_dir.rglob("*") if f.is_file()]

            scripts_dir = skill_dir / "scripts"
            if scripts_dir.exists():
                for ext in ["*.py", "*.sh", "*.bash", "*.js"]:
                    script_files.extend([str(f.relative_to(skill_dir)) for f in scripts_dir.glob(ext)])

        linked_files = {}
        if reference_files:
            linked_files["references"] = reference_files
        if template_files:
            linked_files["templates"] = template_files
        if asset_files:
            linked_files["assets"] = asset_files
        if script_files:
            linked_files["scripts"] = script_files

        # Get skill name from frontmatter or directory
        skill_name = frontmatter_data.get("name", skill_dir.name if skill_dir else name)

        # Get tags
        tags = frontmatter_data.get("tags", [])
        if isinstance(tags, str):
            tags = [t.strip() for t in tags.split(",") if t.strip()]

        related_skills = frontmatter_data.get("related_skills", [])
        if isinstance(related_skills, str):
            related_skills = [s.strip() for s in related_skills.split(",") if s.strip()]

        result = {
            "success": True,
            "name": skill_name,
            "description": frontmatter_data.get("description", ""),
            "tags": tags,
            "related_skills": related_skills,
            "content": content,
            "linked_files": linked_files if linked_files else None,
            "version": frontmatter_data.get("version", "1.0.0"),
        }

        return json.dumps(result, ensure_ascii=False)

    except Exception as e:
        logger.error("skill_view failed: %s", e)
        return json.dumps({"success": False, "error": str(e)}, ensure_ascii=False)


def _skills_categories() -> str:
    """List available skill categories with descriptions."""
    try:
        skills_dirs = _get_skills_dirs()

        if not skills_dirs:
            return json.dumps({
                "success": True,
                "categories": [],
                "message": "No skills directory found.",
            }, ensure_ascii=False)

        category_dirs = {}
        category_counts = {}

        for scan_dir in skills_dirs:
            for skill_md in scan_dir.rglob("SKILL.md"):
                if any(excluded in skill_md.parts for excluded in _EXCLUDED_SKILL_DIRS):
                    continue

                try:
                    content = skill_md.read_text(encoding="utf-8")[:4000]
                    frontmatter_data, _ = _parse_frontmatter(content)
                except Exception:
                    frontmatter_data = {}

                if not _skill_matches_platform(frontmatter_data):
                    continue

                category = _get_category_from_path(skill_md)
                if category:
                    category_counts[category] = category_counts.get(category, 0) + 1
                    if category not in category_dirs:
                        category_dirs[category] = skill_md.parent.parent

        categories = []
        for name in sorted(category_dirs.keys()):
            cat_entry = {
                "name": name,
                "skill_count": category_counts[name],
            }

            # Try to load description from category
            desc_file = category_dirs[name] / "DESCRIPTION.md"
            if desc_file.exists():
                try:
                    cat_content = desc_file.read_text(encoding="utf-8")
                    _, cat_body = _parse_frontmatter(cat_content)
                    # Use first non-header line as description
                    for line in cat_body.strip().split("\n"):
                        line = line.strip()
                        if line and not line.startswith("#"):
                            cat_entry["description"] = line[:MAX_DESCRIPTION_LENGTH]
                            break
                except Exception:
                    pass

            categories.append(cat_entry)

        return json.dumps({
            "success": True,
            "categories": categories,
            "hint": "Use skills_list(category) to see skills in a category",
        }, ensure_ascii=False)

    except Exception as e:
        return json.dumps({"success": False, "error": str(e)}, ensure_ascii=False)


def _skill_create(
    name: str,
    description: str,
    content: str,
    version: str = "1.0.0",
    author: str = "",
) -> str:
    """Create a new skill.

    Args:
        name: Skill name (will be directory name)
        description: Skill description
        content: Skill content (markdown)
        version: Skill version
        author: Author name

    Returns:
        JSON result
    """
    try:
        manager = get_skill_manager()
        result = manager.create_skill(
            name=name,
            description=description,
            content=content,
            version=version,
            author=author,
        )
        return json.dumps(result, ensure_ascii=False)
    except Exception as e:
        logger.error("skill_create failed: %s", e)
        return json.dumps({"success": False, "error": str(e)}, ensure_ascii=False)


def _skill_patch(
    name: str,
    description: str = None,
    content: str = None,
    version: str = None,
    author: str = None,
) -> str:
    """Patch a skill's metadata or content.

    Args:
        name: Skill name
        description: New description (if provided)
        content: New content (if provided)
        version: New version (if provided)
        author: New author (if provided)

    Returns:
        JSON result
    """
    try:
        manager = get_skill_manager()
        result = manager.patch_skill(
            name=name,
            description=description,
            content=content,
            version=version,
            author=author,
        )
        return json.dumps(result, ensure_ascii=False)
    except Exception as e:
        logger.error("skill_patch failed: %s", e)
        return json.dumps({"success": False, "error": str(e)}, ensure_ascii=False)


def _skill_delete(name: str) -> str:
    """Delete a skill.

    Args:
        name: Skill name

    Returns:
        JSON result
    """
    try:
        manager = get_skill_manager()
        result = manager.delete_skill(name)
        return json.dumps(result, ensure_ascii=False)
    except Exception as e:
        logger.error("skill_delete failed: %s", e)
        return json.dumps({"success": False, "error": str(e)}, ensure_ascii=False)


def _load_skill(skill_name: str) -> str:
    """Load the full content of a skill by name.

    Args:
        skill_name: Name of the skill to load

    Returns:
        JSON with skill content
    """
    try:
        manager = get_skill_manager()
        skill = manager.get_skill(skill_name)

        if not skill:
            return json.dumps({
                "success": False,
                "error": f"Skill '{skill_name}' not found. Use skills_list to see available skills.",
            }, ensure_ascii=False)

        return json.dumps({
            "success": True,
            "name": skill["name"],
            "description": skill["description"],
            "content": skill["content"],
            "version": skill.get("version", "1.0.0"),
        }, ensure_ascii=False)

    except Exception as e:
        logger.error("load_skill failed: %s", e)
        return json.dumps({"success": False, "error": str(e)}, ensure_ascii=False)


def register_tools() -> None:
    """Register skills tools."""
    registry = get_registry()

    # skills_list tool
    registry.register(
        name="skills_list",
        description="List available skills with metadata (name + description). Use skill_view(name) to load full content.",
        schema={
            "type": "object",
            "properties": {
                "category": {
                    "type": "string",
                    "description": "Optional category filter to narrow results",
                },
            },
            "required": [],
        },
        handler=lambda **kwargs: _skills_list(category=kwargs.get("category")),
    )

    # skill_view tool
    registry.register(
        name="skill_view",
        description="View the content of a skill or a specific file within a skill. First call returns SKILL.md content plus available linked_files. Use file_path to access specific files (e.g., 'references/api.md').",
        schema={
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "The skill name (use skills_list to see available skills)",
                },
                "file_path": {
                    "type": "string",
                    "description": "Optional path to a linked file within the skill (e.g., 'references/api.md', 'templates/config.yaml')",
                },
            },
            "required": ["name"],
        },
        handler=lambda **kwargs: _skill_view(
            name=kwargs.get("name", ""),
            file_path=kwargs.get("file_path"),
        ),
    )

    # skills_categories tool
    registry.register(
        name="skills_categories",
        description="List available skill categories with descriptions and skill counts.",
        schema={
            "type": "object",
            "properties": {},
            "required": [],
        },
        handler=lambda **kwargs: _skills_categories(),
    )

    # skill_create tool
    registry.register(
        name="skill_create",
        description="Create a new skill with SKILL.md file. Returns error if skill already exists.",
        schema={
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "Skill name (will be directory name)",
                },
                "description": {
                    "type": "string",
                    "description": "Skill description",
                },
                "content": {
                    "type": "string",
                    "description": "Skill content (markdown)",
                },
                "version": {
                    "type": "string",
                    "description": "Skill version (default: 1.0.0)",
                },
                "author": {
                    "type": "string",
                    "description": "Author name",
                },
            },
            "required": ["name", "description", "content"],
        },
        handler=lambda **kwargs: _skill_create(
            name=kwargs.get("name", ""),
            description=kwargs.get("description", ""),
            content=kwargs.get("content", ""),
            version=kwargs.get("version", "1.0.0"),
            author=kwargs.get("author", ""),
        ),
    )

    # skill_patch tool
    registry.register(
        name="skill_patch",
        description="Patch a skill's metadata or content. Only provided fields will be updated.",
        schema={
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "Skill name to patch",
                },
                "description": {
                    "type": "string",
                    "description": "New description (optional)",
                },
                "content": {
                    "type": "string",
                    "description": "New content (optional)",
                },
                "version": {
                    "type": "string",
                    "description": "New version (optional)",
                },
                "author": {
                    "type": "string",
                    "description": "New author (optional)",
                },
            },
            "required": ["name"],
        },
        handler=lambda **kwargs: _skill_patch(
            name=kwargs.get("name", ""),
            description=kwargs.get("description"),
            content=kwargs.get("content"),
            version=kwargs.get("version"),
            author=kwargs.get("author"),
        ),
    )

    # skill_delete tool
    registry.register(
        name="skill_delete",
        description="Delete a skill and its directory. Returns error if skill doesn't exist.",
        schema={
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "Skill name to delete",
                },
            },
            "required": ["name"],
        },
        handler=lambda **kwargs: _skill_delete(name=kwargs.get("name", "")),
    )

    # load_skill tool
    registry.register(
        name="load_skill",
        description="Load the full content of a skill by name. Returns the complete SKILL.md content including instructions and examples. Use this when you need to see the complete skill content.",
        schema={
            "type": "object",
            "properties": {
                "skill_name": {
                    "type": "string",
                    "description": "The name of the skill to load (e.g., 'csv-data-analysis', 'pptx')",
                },
            },
            "required": ["skill_name"],
        },
        handler=lambda **kwargs: _load_skill(skill_name=kwargs.get("skill_name", "")),
    )