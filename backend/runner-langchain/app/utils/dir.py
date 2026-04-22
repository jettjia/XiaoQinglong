"""Directory management following Go runner's xqldir design.

Environment variables (priority high to low):
1. RUNNER_HOME - Complete profile isolation
2. XQL_BASE_DIR - Base directory override
3. ~/.xiaoqinglong - Default

Directory structure:
~/.xiaoqinglong/
├── skills/           # User skills
├── config/           # Configuration files
├── logs/             # Log files
├── checkpoints/      # Execution checkpoints
├── memory/           # Memory storage
│   ├── sessions/    # Session memory
│   ├── users/       # User memory
│   └── agents/      # Agent memory
├── data/
│   ├── uploads/      # Uploaded files
│   └── reports/      # Generated reports
"""

import os
import shutil
import logging
from pathlib import Path

logger = logging.getLogger(__name__)

# Environment variable names (matching Go runner)
BASE_DIR_ENV = "XQL_BASE_DIR"
RUNNER_HOME_ENV = "RUNNER_HOME"

# Default base directory name
DEFAULT_BASE_DIR = ".xiaoqinglong"

# Source skills directory (relative to runner binary, for dev)
_SOURCE_SKILLS_DIR = ""


def get_base_dir() -> Path:
    """Get the unified base directory path.

    Priority:
    1. RUNNER_HOME (highest, for profile isolation)
    2. XQL_BASE_DIR
    3. ~/.xiaoqinglong (default)

    Returns:
        Path to base directory
    """
    # 1. Check RUNNER_HOME (highest priority)
    runner_home = os.environ.get(RUNNER_HOME_ENV)
    if runner_home:
        if os.path.isabs(runner_home):
            return Path(runner_home)
        # Resolve relative paths relative to cwd
        return Path.cwd() / runner_home

    # 2. Check XQL_BASE_DIR
    base_dir = os.environ.get(BASE_DIR_ENV)
    if base_dir:
        if os.path.isabs(base_dir):
            return Path(base_dir)
        # Resolve relative paths relative to home
        return Path.home() / base_dir

    # 3. Default to ~/.xiaoqinglong
    home = os.path.expanduser("~")
    if home == "~":
        # Fallback to /tmp if home cannot be determined
        return Path("/tmp") / DEFAULT_BASE_DIR
    return Path(home) / DEFAULT_BASE_DIR


def get_skills_dir() -> Path:
    """Get the skills directory path."""
    return get_base_dir() / "skills"


def get_uploads_dir() -> Path:
    """Get the uploads directory path."""
    return get_base_dir() / "data" / "uploads"


def get_reports_dir() -> Path:
    """Get the reports directory path."""
    return get_base_dir() / "data" / "reports"


def get_logs_dir() -> Path:
    """Get the logs directory path."""
    return get_base_dir() / "logs"


def get_checkpoints_dir() -> Path:
    """Get the checkpoints directory path."""
    return get_base_dir() / "checkpoints"


def get_config_dir() -> Path:
    """Get the config directory path."""
    return get_base_dir() / "config"


def get_memory_dir() -> Path:
    """Get the memory directory path."""
    return get_base_dir() / "memory"


def get_session_memory_dir(session_id: str) -> Path:
    """Get session memory directory."""
    return get_memory_dir() / "sessions" / session_id


def get_user_memory_dir(user_id: str) -> Path:
    """Get user memory directory."""
    return get_memory_dir() / "users" / user_id


def get_agent_memory_dir(agent_id: str) -> Path:
    """Get agent memory directory."""
    return get_memory_dir() / "agents" / agent_id


def ensure_base_dir() -> None:
    """Ensure base directory and all subdirectories exist."""
    dirs = [
        get_base_dir(),
        get_skills_dir(),
        get_uploads_dir(),
        get_reports_dir(),
        get_logs_dir(),
        get_checkpoints_dir(),
        get_config_dir(),
        get_memory_dir(),
    ]

    for d in dirs:
        try:
            d.mkdir(parents=True, exist_ok=True)
        except OSError as e:
            logger.warning("[xqldir] Failed to create directory %s: %s", d, e)


def _is_symlink(path: Path) -> bool:
    """Check if path is a symlink."""
    try:
        return path.is_symlink()
    except OSError:
        return False


def _copy_dir(src: Path, dst: Path) -> None:
    """Copy directory recursively."""
    if not src.exists():
        raise FileNotFoundError(f"Source directory not found: {src}")

    dst.mkdir(parents=True, exist_ok=True)

    for item in src.rglob("*"):
        if item.is_dir():
            dst_dir = dst / item.relative_to(src)
            dst_dir.mkdir(parents=True, exist_ok=True)
        else:
            dst_file = dst / item.relative_to(src)
            dst_file.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(item, dst_file)


def _copy_file(src: Path, dst: Path) -> None:
    """Copy a single file."""
    dst.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(src, dst)


def _is_dir_empty(path: Path) -> bool:
    """Check if directory is empty."""
    if not path.exists():
        return True
    try:
        return not any(path.iterdir())
    except OSError:
        return True


def ensure_skills_dir() -> None:
    """Ensure skills directory exists and copy default skills if needed.

    If skills dir is a symlink, remove it.
    If skills dir doesn't exist or is empty, copy from SourceSkillsDir.
    """
    skills_dir = get_skills_dir()

    # Check if it's a symlink
    if _is_symlink(skills_dir):
        logger.warning("[xqldir] Removing invalid symlink: %s", skills_dir)
        try:
            skills_dir.unlink()
        except OSError as e:
            logger.warning("[xqldir] Failed to remove symlink: %s", e)
            return

    # Check if needs copy
    needs_copy = False
    if not skills_dir.exists():
        needs_copy = True
    elif _is_dir_empty(skills_dir):
        needs_copy = True

    if needs_copy and _SOURCE_SKILLS_DIR:
        src = Path(_SOURCE_SKILLS_DIR)
        if src.exists():
            logger.info("[xqldir] Copying default skills from %s to %s", src, skills_dir)
            # Remove existing empty dir
            if skills_dir.exists():
                shutil.rmtree(skills_dir)
            try:
                _copy_dir(src, skills_dir)
            except Exception as e:
                logger.warning("[xqldir] Failed to copy default skills: %s", e)
                skills_dir.mkdir(parents=True, exist_ok=True)
        else:
            logger.info("[xqldir] Source skills dir not found: %s", src)
            skills_dir.mkdir(parents=True, exist_ok=True)
    elif needs_copy:
        skills_dir.mkdir(parents=True, exist_ok=True)


def ensure_config_files() -> None:
    """Ensure config files exist in config directory."""
    config_dir = get_config_dir()
    config_dir.mkdir(parents=True, exist_ok=True)

    # Copy skills-config.yaml if not exists
    skills_config = config_dir / "skills-config.yaml"
    if not skills_config.exists() and _SOURCE_SKILLS_DIR:
        src_config = Path(_SOURCE_SKILLS_DIR).parent / "skills-config.yaml"
        if src_config.exists():
            try:
                _copy_file(src_config, skills_config)
                logger.info("[xqldir] Copied skills-config.yaml to %s", skills_config)
            except Exception as e:
                logger.warning("[xqldir] Failed to copy skills-config.yaml: %s", e)


def init() -> None:
    """Initialize directory structure.

    Should be called at runner startup.
    """
    ensure_base_dir()
    logger.info("[xqldir] Base directory initialized: %s", get_base_dir())

    ensure_skills_dir()
    ensure_config_files()


def set_source_skills_dir(path: str) -> None:
    """Set the source skills directory (for development).

    This is used to copy default skills to the user's .xiaoqinglong/skills
    on first init.
    """
    global _SOURCE_SKILLS_DIR
    _SOURCE_SKILLS_DIR = path


def get_source_skills_dir() -> str:
    """Get the source skills directory."""
    return _SOURCE_SKILLS_DIR