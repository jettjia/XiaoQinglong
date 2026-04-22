"""Skill CRUD manager.

Handles create/read/update/delete operations for skills.
"""

import json
import logging
import shutil
from pathlib import Path
from typing import Any, Optional

import frontmatter

from app.skills.loader import Skill, SkillLoader, SkillMetadata
from app.utils.dir import get_skills_dir

logger = logging.getLogger(__name__)


class SkillManager:
    """Manages skill CRUD operations.

    Skills are stored as directories with SKILL.md files.
    """

    def __init__(self, skills_dir: Optional[Path] = None):
        self.skills_dir = skills_dir or get_skills_dir()
        self.loader = SkillLoader([self.skills_dir])

    def list_skills(self) -> list[dict[str, Any]]:
        """List all available skills.

        Returns:
            List of skill info dicts
        """
        skills = self.loader.discover_skills()
        return [
            {
                "name": s.name,
                "description": s.description,
                "version": s.metadata.version,
                "path": str(s.path),
                "risk_level": s.metadata.get_risk_level(),
                "tags": s.metadata.get_tags(),
            }
            for s in skills
        ]

    def get_skill(self, name: str) -> Optional[dict[str, Any]]:
        """Get a skill by name.

        Args:
            name: Skill name

        Returns:
            Skill info dict or None
        """
        skill = self.loader.get_skill(name)
        if not skill:
            return None
        return skill.to_dict()

    def create_skill(
        self,
        name: str,
        description: str,
        content: str,
        version: str = "1.0.0",
        author: str = "",
        metadata: Optional[dict[str, Any]] = None,
    ) -> dict[str, Any]:
        """Create a new skill.

        Args:
            name: Skill name (will be directory name)
            description: Skill description
            content: Skill content (markdown)
            version: Skill version
            author: Author name
            metadata: Additional metadata

        Returns:
            Result dict with success/error
        """
        if not name or not description:
            return {"success": False, "error": "name and description are required"}

        # Check if skill already exists
        existing = self.loader.get_skill(name)
        if existing:
            return {"success": False, "error": f"Skill '{name}' already exists"}

        skill_dir = self.skills_dir / name
        try:
            skill_dir.mkdir(parents=True, exist_ok=True)

            frontmatter_data = {
                "name": name,
                "description": description,
                "version": version,
            }
            if author:
                frontmatter_data["author"] = author
            if metadata:
                frontmatter_data["metadata"] = metadata

            skill_path = skill_dir / "SKILL.md"
            post = frontmatter.Post(content, frontmatter_data)
            with open(skill_path, "w", encoding="utf-8") as f:
                frontmatter.write(post, f)

            logger.info("[SkillManager] Created skill '%s' at %s", name, skill_path)

            return {
                "success": True,
                "skill": {
                    "name": name,
                    "description": description,
                    "version": version,
                    "path": str(skill_path),
                },
            }

        except Exception as e:
            logger.warning("[SkillManager] Failed to create skill '%s': %s", name, e)
            # Cleanup on failure
            if skill_dir.exists():
                shutil.rmtree(skill_dir, ignore_errors=True)
            return {"success": False, "error": str(e)}

    def patch_skill(
        self,
        name: str,
        description: Optional[str] = None,
        content: Optional[str] = None,
        version: Optional[str] = None,
        author: Optional[str] = None,
    ) -> dict[str, Any]:
        """Patch a skill's metadata or content.

        Args:
            name: Skill name
            description: New description (if provided)
            content: New content (if provided)
            version: New version (if provided)
            author: New author (if provided)

        Returns:
            Result dict with success/error
        """
        skill = self.loader.get_skill(name)
        if not skill:
            return {"success": False, "error": f"Skill '{name}' not found"}

        try:
            # Read current frontmatter
            post = frontmatter.read(skill.path)

            # Update fields
            if description is not None:
                post.metadata["description"] = description
            if version is not None:
                post.metadata["version"] = version
            if author is not None:
                post.metadata["author"] = author
            if content is not None:
                post.content = content

            # Write back
            with open(skill.path, "w", encoding="utf-8") as f:
                frontmatter.write(post, f)

            logger.info("[SkillManager] Patched skill '%s'", name)

            return {
                "success": True,
                "skill": {
                    "name": name,
                    "description": post.metadata.get("description"),
                    "version": post.metadata.get("version"),
                    "path": str(skill.path),
                },
            }

        except Exception as e:
            logger.warning("[SkillManager] Failed to patch skill '%s': %s", name, e)
            return {"success": False, "error": str(e)}

    def delete_skill(self, name: str) -> dict[str, Any]:
        """Delete a skill.

        Args:
            name: Skill name

        Returns:
            Result dict with success/error
        """
        skill = self.loader.get_skill(name)
        if not skill:
            return {"success": False, "error": f"Skill '{name}' not found"}

        try:
            skill_dir = skill.path.parent
            shutil.rmtree(skill_dir)

            logger.info("[SkillManager] Deleted skill '%s' at %s", name, skill_dir)

            return {"success": True, "message": f"Skill '{name}' deleted"}

        except Exception as e:
            logger.warning("[SkillManager] Failed to delete skill '%s': %s", name, e)
            return {"success": False, "error": str(e)}


# Global manager instance
_skill_manager: Optional[SkillManager] = None


def get_skill_manager() -> SkillManager:
    """Get the global skill manager instance."""
    global _skill_manager
    if _skill_manager is None:
        _skill_manager = SkillManager()
    return _skill_manager