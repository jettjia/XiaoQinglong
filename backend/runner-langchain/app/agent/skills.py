"""Skills system prompt integration.

Based on hermes-agent's prompt_builder.py skills section design.
"""

import hashlib
import json
import logging
import os
from pathlib import Path
from typing import Optional

from app.skills.loader import SkillLoader, Skill

logger = logging.getLogger(__name__)

# Default skills directories
DEFAULT_SKILLS_DIRS = [
    Path.home() / ".runner-langchain" / "skills",
    Path("~/.claude/skills").expanduser(),
]

# Cache file for skills snapshot
SKILLS_CACHE_FILE = Path.home() / ".runner-langchain" / "skills_snapshot.json"


class SkillsSystemPromptBuilder:
    """Builds skills section for system prompt.

    Uses a two-layer cache (in-process LRU + disk snapshot) similar to hermes-agent.
    """

    def __init__(self, skills_dirs: list[Path] | None = None):
        self.loader = SkillLoader(skills_dirs or DEFAULT_SKILLS_DIRS.copy())
        self._cache: dict[str, str] = {}
        self._cache_mtime: dict[str, float] = {}

    def _get_skill_hash(self, skill: Skill) -> str:
        """Get hash for a skill based on path and mtime."""
        try:
            mtime = os.path.getmtime(skill.path)
            key = f"{skill.path}:{mtime}"
            return hashlib.md5(key.encode()).hexdigest()[:12]
        except OSError:
            return hashlib.md5(skill.path.name.encode()).hexdigest()[:12]

    def _load_skills_snapshot(self) -> dict[str, str]:
        """Load skills snapshot from disk cache."""
        if not SKILLS_CACHE_FILE.exists():
            return {}

        try:
            with open(SKILLS_CACHE_FILE) as f:
                return json.load(f)
        except (json.JSONDecodeError, IOError):
            return {}

    def _save_skills_snapshot(self, snapshot: dict[str, str]) -> None:
        """Save skills snapshot to disk cache."""
        try:
            SKILLS_CACHE_FILE.parent.mkdir(parents=True, exist_ok=True)
            with open(SKILLS_CACHE_FILE, "w") as f:
                json.dump(snapshot, f)
        except IOError as e:
            logger.warning("Failed to save skills snapshot: %s", e)

    def _build_skills_manifest(self, skills: list[Skill]) -> dict[str, dict]:
        """Build manifest of skill names to (mtime, size) pairs."""
        manifest = {}
        for skill in skills:
            try:
                stat = skill.path.stat()
                manifest[skill.name] = {
                    "mtime": stat.st_mtime,
                    "size": stat.st_size,
                }
            except OSError:
                continue
        return manifest

    def _skills_changed(self, manifest: dict[str, dict]) -> bool:
        """Check if any skills have changed since last snapshot."""
        snapshot = self._load_skills_snapshot()
        if not snapshot:
            return True

        for name, info in manifest.items():
            if name not in snapshot:
                return True
            if abs(snapshot[name].get("mtime", 0) - info["mtime"]) > 0.001:
                return True

        return False

    def build_skills_system_prompt(
        self,
        available_tools: list[str] | None = None,
        toolsets: list[str] | None = None,
        force_reload: bool = False,
    ) -> str:
        """Build skills section for system prompt.

        Args:
            available_tools: List of available tool names
            toolsets: List of toolset names to filter skills
            force_reload: Force reload skills from disk

        Returns:
            Skills section string for system prompt
        """
        skills = self.loader.discover_skills()

        if not skills:
            return ""

        # Build manifest and check if we need to rebuild cache
        manifest = self._build_skills_manifest(skills)

        if force_reload or self._skills_changed(manifest):
            # Rebuild cache
            self._cache = {}
            self._cache_mtime = {}

            for skill in skills:
                skill_hash = self._get_skill_hash(skill)
                content = self._format_skill_entry(skill, available_tools, toolsets)
                self._cache[skill.name] = content
                try:
                    self._cache_mtime[skill.name] = os.path.getmtime(skill.path)
                except OSError:
                    self._cache_mtime[skill.name] = 0

            # Save snapshot
            self._save_skills_snapshot(manifest)
        else:
            # Verify cache is still valid
            for skill in skills:
                try:
                    current_mtime = os.path.getmtime(skill.path)
                    cached_mtime = self._cache_mtime.get(skill.name, -1)
                    if abs(current_mtime - cached_mtime) > 0.001:
                        # Skill changed, rebuild entry
                        skill_hash = self._get_skill_hash(skill)
                        self._cache[skill.name] = self._format_skill_entry(
                            skill, available_tools, toolsets
                        )
                        self._cache_mtime[skill.name] = current_mtime
                except OSError:
                    continue

        # Build final output
        active_skills = [s for s in skills if self._is_skill_active(s, available_tools, toolsets)]

        if not active_skills:
            return ""

        header = f"[AVAILABLE SKILLS — {len(active_skills)} skills loaded]"
        entries = []

        for skill in active_skills:
            entry = self._cache.get(skill.name, "")
            if entry:
                entries.append(entry)

        if not entries:
            return ""

        return f"\n\n{header}\n" + "\n".join(entries) + "\n[/AVAILABLE SKILLS]"

    def _is_skill_active(
        self,
        skill: Skill,
        available_tools: list[str] | None,
        toolsets: list[str] | None,
    ) -> bool:
        """Check if a skill is active given available tools and toolsets."""
        # If no filters, all skills are active
        if not available_tools and not toolsets:
            return True

        # Check toolsets filter
        if toolsets:
            skill_toolsets = skill.metadata.get("tags", [])
            if not any(ts in skill_toolsets for ts in toolsets):
                return False

        return True

    def _format_skill_entry(
        self,
        skill: Skill,
        available_tools: list[str] | None,
        toolsets: list[str] | None,
    ) -> str:
        """Format a single skill entry."""
        name = skill.name
        desc = skill.description or "No description"

        # Build constraints line if needed
        constraints = []
        if available_tools:
            constraints.append(f"tools: {', '.join(available_tools[:5])}")
        if toolsets:
            constraints.append(f"toolsets: {', '.join(toolsets)}")

        constraint_line = f" [{', '.join(constraints)}]" if constraints else ""

        return f"""
## {name}
{description}
{constraint_line}
"""


# Global instance
_skills_builder: Optional[SkillsSystemPromptBuilder] = None


def get_skills_builder() -> SkillsSystemPromptBuilder:
    """Get the global skills system prompt builder."""
    global _skills_builder
    if _skills_builder is None:
        _skills_builder = SkillsSystemPromptBuilder()
    return _skills_builder


def build_skills_system_prompt(
    available_tools: list[str] | None = None,
    toolsets: list[str] | None = None,
    force_reload: bool = False,
) -> str:
    """Build skills section for system prompt.

    Convenience function that uses the global builder.
    """
    return get_skills_builder().build_skills_system_prompt(
        available_tools=available_tools,
        toolsets=toolsets,
        force_reload=force_reload,
    )