package xqldir

import (
	"os"
	"path/filepath"
)

const (
	// BaseDirEnv is the environment variable for the base directory
	BaseDirEnv = "XQL_BASE_DIR"
	// DefaultBaseDir is the default base directory name (in home directory)
	DefaultBaseDir = ".xiaoqinglong"
)

// GetBaseDir 获取基础目录
func GetBaseDir() string {
	// 兼容旧的 APP_DATA 环境变量
	if baseDir := os.Getenv("APP_DATA"); baseDir != "" {
		return baseDir
	}

	// 优先使用 XQL_BASE_DIR 环境变量
	if baseDir := os.Getenv(BaseDirEnv); baseDir != "" {
		if filepath.IsAbs(baseDir) {
			return baseDir
		}
		// 相对路径相对于 home 目录
		home, _ := os.UserHomeDir()
		return filepath.Join(home, baseDir)
	}

	// 默认使用 ~/.xiaoqinglong
	home, _ := os.UserHomeDir()
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

// EnsureBaseDir ensures the base directory and all subdirectories exist
func EnsureBaseDir() error {
	dirs := []string{
		GetBaseDir(),
		GetSkillsDir(),
		GetUploadsDir(),
		GetReportsDir(),
		GetLogsDir(),
		GetCheckpointsDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}