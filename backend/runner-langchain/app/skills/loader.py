"""Skill loader based on hermes-agent design.

Skills are defined as SKILL.md files with YAML frontmatter:
---
name: skill-name
description: Skill description
version: 1.0.0
---

# Skill Title

Skill content with usage instructions...
"""

import os
from pathlib import Path
from typing import Any, Optional
import frontmatter


class SkillMetadata:
    """Skill metadata parsed from SKILL.md frontmatter."""

    def __init__(self, data: dict[str, Any]):
        self.name: str = data.get("name", "")
        self.description: str = data.get("description", "")
        self.version: str = data.get("version", "1.0.0")
        self.author: str = data.get("author", "")
        self.license: str = data.get("license", "")
        self.prerequisites: dict[str, Any] = data.get("prerequisites", {})
        self.metadata: dict[str, Any] = data.get("metadata", {})

    def get_risk_level(self) -> str:
        """Get risk level from metadata."""
        hermes = self.metadata.get("hermes", {})
        return hermes.get("risk_level", "medium")

    def get_tags(self) -> list[str]:
        """Get tags from metadata."""
        hermes = self.metadata.get("hermes", {})
        return hermes.get("tags", [])


class Skill:
    """Represents a loaded skill."""

    def __init__(
        self,
        path: Path,
        metadata: SkillMetadata,
        content: str,
    ):
        self.path = path
        self.metadata = metadata
        self.content = content
        self.name = metadata.name
        self.description = metadata.description

    @property
    def directory(self) -> Path:
        """Get skill directory."""
        return self.path.parent

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        return {
            "name": self.name,
            "description": self.description,
            "version": self.metadata.version,
            "path": str(self.path),
            "content": self.content,
            "risk_level": self.metadata.get_risk_level(),
            "tags": self.metadata.get_tags(),
        }


class SkillLoader:
    """Loads skills from SKILL.md files.

    Based on hermes-agent's skill loading design.
    """

    SKILL_FILENAME = "SKILL.md"
    EXCLUDED_DIRS = frozenset((".git", ".github", ".hub", ".venv", "node_modules"))

    def __init__(self, skills_dirs: list[Path] | None = None):
        """Initialize skill loader.

        Args:
            skills_dirs: List of directories to search for skills.
                        If None, uses default locations.
        """
        self._skills_dirs = skills_dirs or []

    def add_skills_dir(self, path: Path) -> None:
        """Add a directory to search for skills."""
        if path not in self._skills_dirs:
            self._skills_dirs.append(path)

    def discover_skills(self) -> list[Skill]:
        """Discover all skills in configured directories.

        Returns:
            List of discovered skills
        """
        skills: list[Skill] = []
        seen_names: set[str] = set()

        for skills_dir in self._skills_dirs:
            if not skills_dir.is_dir():
                continue

            for skill in self._discover_in_dir(skills_dir):
                if skill.name not in seen_names:
                    skills.append(skill)
                    seen_names.add(skill.name)

        return skills

    def _discover_in_dir(self, skills_dir: Path) -> list[Skill]:
        """Discover skills in a single directory."""
        skills: list[Skill] = []

        for root, dirs, files in os.walk(skills_dir):
            # Filter out excluded directories
            dirs[:] = [d for d in dirs if d not in self.EXCLUDED_DIRS]

            if self.SKILL_FILENAME in files:
                skill_path = Path(root) / self.SKILL_FILENAME
                try:
                    skill = self.load_skill(skill_path)
                    if skill:
                        skills.append(skill)
                except Exception:
                    continue

        return skills

    def load_skill(self, path: Path) -> Optional[Skill]:
        """Load a single skill from SKILL.md file.

        Args:
            path: Path to SKILL.md file

        Returns:
            Loaded Skill or None if failed
        """
        try:
            post = frontmatter.read(path)
            metadata = SkillMetadata(post.metadata)
            content = post.content

            if not metadata.name:
                metadata.name = path.parent.name

            return Skill(path, metadata, content)

        except Exception:
            return None

    def get_skill(self, name: str) -> Optional[Skill]:
        """Get a skill by name.

        Args:
            name: Skill name

        Returns:
            Skill or None if not found
        """
        for skills_dir in self._skills_dirs:
            if not skills_dir.is_dir():
                continue

            # Search in subdirectories
            for item in skills_dir.iterdir():
                if item.is_dir() and item.name == name:
                    skill_path = item / self.SKILL_FILENAME
                    if skill_path.exists():
                        return self.load_skill(skill_path)

            # Also check direct SKILL.md with matching name
            direct_path = skills_dir / f"{name}.md"
            if direct_path.exists():
                return self.load_skill(direct_path)

        return None
