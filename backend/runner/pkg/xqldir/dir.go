package xqldir

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// SourceSkillsDir is the source skills directory (relative to runner binary)
// This is used to copy default skills to the user's .xiaoqinglong/skills on first init
var SourceSkillsDir = ""

// SourceConfigDir is the source config directory (relative to runner binary)
// This is used to copy default config files to the user's .xiaoqinglong/config on first init
var SourceConfigDir = ""

// BaseDirEnv is the environment variable for the base directory
const BaseDirEnv = "XQL_BASE_DIR"

// RunnerHomeEnv is the environment variable for RUNNER_HOME
// This allows complete profile isolation between different runner instances
const RunnerHomeEnv = "RUNNER_HOME"

// DefaultBaseDir is the default base directory name (in home directory)
const DefaultBaseDir = ".xiaoqinglong"

// GetBaseDir returns the unified base directory path
// Priority: RUNNER_HOME > XQL_BASE_DIR > ~/.xiaoqinglong
func GetBaseDir() string {
	// 1. Check RUNNER_HOME environment variable (highest priority, for profile isolation)
	if runnerHome := os.Getenv(RunnerHomeEnv); runnerHome != "" {
		if filepath.IsAbs(runnerHome) {
			return runnerHome
		}
		// Resolve relative paths relative to current working directory
		cwd, _ := os.Getwd()
		return filepath.Join(cwd, runnerHome)
	}

	// 2. Check XQL_BASE_DIR environment variable
	if baseDir := os.Getenv(BaseDirEnv); baseDir != "" {
		if filepath.IsAbs(baseDir) {
			return baseDir
		}
		// Resolve relative paths relative to home directory
		home, _ := os.UserHomeDir()
		return filepath.Join(home, baseDir)
	}

	// 3. Default to ~/.xiaoqinglong
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to /tmp if home cannot be determined
		return filepath.Join("/tmp", DefaultBaseDir)
	}
	return filepath.Join(home, DefaultBaseDir)
}

// GetSkillsDir returns the skills directory path
func GetSkillsDir() string {
	return filepath.Join(GetBaseDir(), "skills")
}

// GetUploadsDir returns the uploads directory path
func GetUploadsDir() string {
	return filepath.Join(GetBaseDir(), "data", "uploads")
}

// GetReportsDir returns the reports directory path
func GetReportsDir() string {
	return filepath.Join(GetBaseDir(), "data", "reports")
}

// GetLogsDir returns the logs directory path
func GetLogsDir() string {
	return filepath.Join(GetBaseDir(), "logs")
}

// GetCheckpointsDir returns the checkpoints directory path
func GetCheckpointsDir() string {
	return filepath.Join(GetBaseDir(), "checkpoints")
}

// GetConfigDir returns the config directory path
func GetConfigDir() string {
	return filepath.Join(GetBaseDir(), "config")
}

// GetMemoryDir returns the memory directory path
func GetMemoryDir() string {
	return filepath.Join(GetBaseDir(), "memory")
}

// GetSessionMemoryDir returns the session memory directory path
func GetSessionMemoryDir(sessionID string) string {
	return filepath.Join(GetMemoryDir(), "sessions", sessionID)
}

// GetUserMemoryDir returns the user memory directory path
func GetUserMemoryDir(userID string) string {
	return filepath.Join(GetMemoryDir(), "users", userID)
}

// GetAgentMemoryDir returns the agent memory directory path
func GetAgentMemoryDir(agentID string) string {
	return filepath.Join(GetMemoryDir(), "agents", agentID)
}

// EnsureBaseDir ensures the base directory and all subdirectories exist
func EnsureBaseDir() error {
	dirs := []string{
		GetBaseDir(),
		GetSkillsDir(),
		GetUploadsDir(),
		GetReportsDir(),
		GetLogsDir(),
		GetCheckpointsDir(),
		GetConfigDir(),
		GetMemoryDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// Init ensures the base directory structure exists
// Should be called at runner startup
func Init() {
	if err := EnsureBaseDir(); err != nil {
		log.Fatalf("[xqldir] Warning: failed to create base directories: %v", err)
	} else {
		log.Printf("[xqldir] Base directory initialized: %s", GetBaseDir())
	}

	// 确保 skills 目录是正确的（不是 symlink），并复制默认 skills
	ensureSkillsDir()

	// 确保配置文件在 ~/.xiaoqinglong/config/ 目录
	ensureConfigFiles()
}

// ensureSkillsDir 确保 skills 目录存在且是真实目录（非 symlink），并复制默认 skills
func ensureSkillsDir() {
	skillsDir := GetSkillsDir()

	// 检查是否是 symlink
	if info, err := os.Lstat(skillsDir); err == nil && info.Mode()&os.ModeSymlink != 0 {
		log.Printf("[xqldir] Removing invalid symlink: %s -> %s", skillsDir, info.Name())
		if err := os.Remove(skillsDir); err != nil {
			log.Printf("[xqldir] Warning: failed to remove symlink: %v", err)
			return
		}
	}

	// 如果目录不存在，或者目录为空，复制默认 skills
	needsCopy := false
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		needsCopy = true
	} else {
		// 检查是否为空目录
		entries, err := os.ReadDir(skillsDir)
		if err != nil || len(entries) == 0 {
			needsCopy = true
		}
	}

	if needsCopy && SourceSkillsDir != "" {
		// 检查源目录是否存在
		if _, srcErr := os.Stat(SourceSkillsDir); srcErr == nil {
			log.Printf("[xqldir] Copying default skills from %s to %s", SourceSkillsDir, skillsDir)
			// 先删除可能存在的空目录
			os.RemoveAll(skillsDir)
			if err := copyDir(SourceSkillsDir, skillsDir); err != nil {
				log.Printf("[xqldir] Warning: failed to copy default skills: %v, creating empty dir", err)
				os.MkdirAll(skillsDir, 0755)
			}
		} else {
			log.Printf("[xqldir] Source skills dir not found: %s, creating empty skills dir", SourceSkillsDir)
			os.MkdirAll(skillsDir, 0755)
		}
	} else if needsCopy {
		os.MkdirAll(skillsDir, 0755)
	}
}

// ensureConfigFiles 确保配置文件在 ~/.xiaoqinglong/config/ 目录
func ensureConfigFiles() {
	configDir := GetConfigDir()

	// 创建配置目录（如果不存在）
	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Printf("[xqldir] Warning: failed to create config dir: %v", err)
		return
	}

	// 复制 skills-config.yaml（如果不存在）
	skillsConfigPath := filepath.Join(configDir, "skills-config.yaml")
	if _, err := os.Stat(skillsConfigPath); os.IsNotExist(err) {
		copySkillsConfig(configDir)
	}

	// 复制 config.yaml（如果不存在）
	configYamlPath := filepath.Join(configDir, "config.yaml")
	if _, err := os.Stat(configYamlPath); os.IsNotExist(err) {
		copyAgentConfig(configDir)
	}
}

func copySkillsConfig(configDir string) {
	// 从 SourceSkillsDir 复制 skills-config.yaml
	if SourceSkillsDir != "" {
		srcSkillsConfig := filepath.Join(SourceSkillsDir, "..", "skills-config.yaml")
		if _, err := os.Stat(srcSkillsConfig); err == nil {
			dstSkillsConfig := filepath.Join(configDir, "skills-config.yaml")
			if err := copyFile(srcSkillsConfig, dstSkillsConfig); err != nil {
				log.Printf("[xqldir] Warning: failed to copy skills-config.yaml: %v", err)
			} else {
				log.Printf("[xqldir] Copied skills-config.yaml to %s", dstSkillsConfig)
			}
		}
	}
}

// GetSourceConfigDir returns the source config directory
// This is used to find config files to copy to ~/.xiaoqinglong/config/
func GetSourceConfigDir() string {
	if SourceConfigDir != "" {
		return SourceConfigDir
	}

	// Fallback: 查找可执行文件同目录下的 config 目录
	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	execDir := filepath.Dir(execPath)
	localConfig := filepath.Join(execDir, "config")
	if _, err := os.Stat(localConfig); err == nil {
		return localConfig
	}

	// Fallback: 查找 dev 路径 (runner 在 build/bin/ 时，config 在 build/bin/config/)
	devConfig := filepath.Join(execDir, "..", "config")
	if _, err := os.Stat(devConfig); err == nil {
		return devConfig
	}

	return ""
}

func copyAgentConfig(configDir string) {
	srcDir := GetSourceConfigDir()
	if srcDir == "" {
		log.Printf("[xqldir] Source config dir not found, skipping config.yaml copy")
		return
	}

	srcConfig := filepath.Join(srcDir, "config.yaml")
	if _, err := os.Stat(srcConfig); err == nil {
		dstConfig := filepath.Join(configDir, "config.yaml")
		if err := copyFile(srcConfig, dstConfig); err != nil {
			log.Printf("[xqldir] Warning: failed to copy config.yaml: %v", err)
		} else {
			log.Printf("[xqldir] Copied config.yaml to %s", dstConfig)
		}
	}
}

// copyDir 复制目录（递归）
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	// 复制权限
	srcInfo, _ := os.Stat(src)
	if srcInfo != nil {
		os.Chmod(dst, srcInfo.Mode())
	}

	return nil
}
