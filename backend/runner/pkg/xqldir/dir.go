package xqldir

import (
	"log"
	"os"
	"path/filepath"
)

// BaseDirEnv is the environment variable for the base directory
const BaseDirEnv = "XQL_BASE_DIR"

// DefaultBaseDir is the default base directory name (in home directory)
const DefaultBaseDir = ".xiaoqinglong"

// GetBaseDir returns the unified base directory path
// Priority: XQL_BASE_DIR env > ~/.xiaoqinglong
func GetBaseDir() string {
	// 1. Check XQL_BASE_DIR environment variable
	if baseDir := os.Getenv(BaseDirEnv); baseDir != "" {
		if filepath.IsAbs(baseDir) {
			return baseDir
		}
		// Resolve relative paths relative to home directory
		home, _ := os.UserHomeDir()
		return filepath.Join(home, baseDir)
	}

	// 2. Default to ~/.xiaoqinglong
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
}
