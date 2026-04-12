package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"
)

//go:embed all:bin/runner.exe
var runnerBinary embed.FS

//go:embed all:bin/skills
var skillsFS embed.FS

//go:embed all:bin/config
var configFS embed.FS

//go:embed all:bin/skills-config.yaml
var skillsConfigFS embed.FS

// ExtractAssets extracts embedded assets to ~/.xiaoqinglong/ on first run
func ExtractAssets() error {
	baseDir := getBaseDir()
	extracted := false

	// Extract runner.exe
	runnerPath := filepath.Join(baseDir, "runner.exe")
	if _, err := os.Stat(runnerPath); os.IsNotExist(err) {
		log.Println("[Embed] Extracting runner.exe...")
		if err := extractFileFromFS(runnerBinary, "bin/runner.exe", runnerPath); err != nil {
			log.Printf("[Embed] Failed to extract runner.exe: %v", err)
		} else {
			log.Printf("[Embed] runner.exe extracted to %s", runnerPath)
			extracted = true
		}
	}

	// Extract skills directory
	skillsDir := filepath.Join(baseDir, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		log.Println("[Embed] Extracting skills directory...")
		if err := extractDirFromFS(skillsFS, "bin/skills", skillsDir); err != nil {
			log.Printf("[Embed] Failed to extract skills: %v", err)
		} else {
			log.Printf("[Embed] skills extracted to %s", skillsDir)
			extracted = true
		}
	} else {
		// Check if skills directory is empty
		entries, err := os.ReadDir(skillsDir)
		if err != nil || len(entries) == 0 {
			log.Println("[Embed] Skills directory is empty, re-extracting...")
			os.RemoveAll(skillsDir)
			if err := extractDirFromFS(skillsFS, "bin/skills", skillsDir); err != nil {
				log.Printf("[Embed] Failed to extract skills: %v", err)
			} else {
				log.Printf("[Embed] skills re-extracted to %s", skillsDir)
				extracted = true
			}
		}
	}

	// Extract config directory
	configDir := filepath.Join(baseDir, "config")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		log.Println("[Embed] Extracting config directory...")
		if err := extractDirFromFS(configFS, "bin/config", configDir); err != nil {
			log.Printf("[Embed] Failed to extract config: %v", err)
		} else {
			log.Printf("[Embed] config extracted to %s", configDir)
			extracted = true
		}
	} else {
		// Check if config.yaml exists
		configYamlPath := filepath.Join(configDir, "config.yaml")
		if _, err := os.Stat(configYamlPath); os.IsNotExist(err) {
			log.Println("[Embed] config.yaml missing, re-extracting...")
			os.RemoveAll(configDir)
			if err := extractDirFromFS(configFS, "bin/config", configDir); err != nil {
				log.Printf("[Embed] Failed to extract config: %v", err)
			} else {
				log.Printf("[Embed] config re-extracted to %s", configDir)
				extracted = true
			}
		}
	}

	// Extract skills-config.yaml
	skillsConfigPath := filepath.Join(baseDir, "skills-config.yaml")
	if _, err := os.Stat(skillsConfigPath); os.IsNotExist(err) {
		log.Println("[Embed] Extracting skills-config.yaml...")
		if err := extractFileFromFS(skillsConfigFS, "bin/skills-config.yaml", skillsConfigPath); err != nil {
			log.Printf("[Embed] Failed to extract skills-config.yaml: %v", err)
		} else {
			log.Printf("[Embed] skills-config.yaml extracted to %s", skillsConfigPath)
			extracted = true
		}
	}

	if extracted {
		log.Println("[Embed] Assets extraction completed")
	}

	return nil
}

// getBaseDir returns the base directory for extracted assets (defined in app.go)
func extractFileFromFS(fs embed.FS, srcPath, dstPath string) error {
	data, err := fs.ReadFile(srcPath)
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(dstPath, data, 0755)
}

// extractDirFromFS extracts a directory from an embed.FS
// Note: embed.FS always uses forward slashes, so we use string concatenation
func extractDirFromFS(fs embed.FS, srcPath, dstPath string) error {
	entries, err := fs.ReadDir(srcPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dstPath, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		// Use forward slash for embed.FS paths (always)
		src := srcPath + "/" + entry.Name()
		dst := filepath.Join(dstPath, entry.Name())

		if entry.IsDir() {
			if err := extractDirFromFS(fs, src, dst); err != nil {
				return err
			}
		} else {
			if err := extractFileFromFS(fs, src, dst); err != nil {
				return err
			}
		}
	}

	return nil
}

// init runs on package import to extract assets
func init() {
	log.Println("[Embed] init() starting...")

	// Write to a test file
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	testFile := home + "/.xiaoqinglong/logs/embed_init.log"
	os.MkdirAll(home+"/.xiaoqinglong/logs", 0755)
	f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		f.WriteString("[Embed] init() running\n")
		f.Close()
	}

	log.Println("[Embed] Checking and extracting embedded assets...")
	if err := ExtractAssets(); err != nil {
		log.Printf("[Embed] Warning: asset extraction error: %v", err)
	}
	log.Println("[Embed] init() completed")
}
