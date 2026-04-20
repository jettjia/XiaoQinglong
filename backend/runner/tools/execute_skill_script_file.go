package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// skillMarkerStorage stores marker data extracted from skill script executions
// Key: skill name, Value: map of marker key -> marker content
// This is used to share marker data between execute_skill_script_file and html_interpreter
// within the same skill workflow.
var skillMarkerStorage = struct {
	data map[string]map[string]string
	mu   sync.RWMutex
}{
	data: make(map[string]map[string]string),
}

// SetSkillMarkers stores marker data for a skill
func SetSkillMarkers(skillName string, markers map[string]string) {
	skillMarkerStorage.mu.Lock()
	defer skillMarkerStorage.mu.Unlock()
	skillMarkerStorage.data[skillName] = markers
}

// GetSkillMarkers retrieves marker data for a skill
func GetSkillMarkers(skillName string) map[string]string {
	skillMarkerStorage.mu.RLock()
	defer skillMarkerStorage.mu.RUnlock()
	if markers, ok := skillMarkerStorage.data[skillName]; ok {
		result := make(map[string]string)
		for k, v := range markers {
			result[k] = v
		}
		return result
	}
	return nil
}

// ClearSkillMarkers removes marker data for a skill
func ClearSkillMarkers(skillName string) {
	skillMarkerStorage.mu.Lock()
	defer skillMarkerStorage.mu.Unlock()
	delete(skillMarkerStorage.data, skillName)
}

// ExecuteSkillScriptInput for execute_skill_script_file tool
type ExecuteSkillScriptInput struct {
	SkillName      string         `json:"skill_name"`                // Skill name (e.g., "csv-data-analysis")
	ScriptFileName string         `json:"script_file_name"`          // Script filename (e.g., "csv_analyzer.py")
	Args           map[string]any `json:"args,omitempty"`           // Arguments to pass to the script
	OutputDir      string         `json:"output_dir,omitempty"`      // Output directory for script execution
}

// ExecuteSkillScriptOutput for execute_skill_script_file result
type ExecuteSkillScriptOutput struct {
	Chunks []SkillScriptChunk `json:"chunks"` // Output chunks
}

// SkillScriptChunk represents a single chunk of script output
type SkillScriptChunk struct {
	OutputType string `json:"output_type"` // "text", "image", "code", "data"
	Content    string `json:"content"`     // Content or path to image
	Key        string `json:"key,omitempty"` // Marker key for data chunks, e.g., "CHART_DATA_JSON"
}

// ExecuteSkillScriptFileTool executes scripts from skills directory
type ExecuteSkillScriptFileTool struct {
	skillsDir string
}

func NewExecuteSkillScriptFileTool(skillsDir string) *ExecuteSkillScriptFileTool {
	return &ExecuteSkillScriptFileTool{
		skillsDir: skillsDir,
	}
}

func init() {
	GlobalRegistry.Register(ToolMeta{
		Name:           "execute_skill_script_file",
		Desc:           "Execute a Python script file from a skill's scripts directory. The script receives args as JSON via sys.argv[1].",
		IsReadOnly:     false,
		MaxResultChars: 500000,
		DefaultRisk:    "medium",
		Creator: func(basePath string) interface{} {
			// Skills dir is resolved from xqldir.GetSkillsDir() at runtime
			return NewExecuteSkillScriptFileTool("")
		},
	})
}

func (t *ExecuteSkillScriptFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "execute_skill_script_file",
		Desc: "Execute a Python script file from a skill's scripts directory. The script receives args as JSON via sys.argv[1]. Returns JSON with chunks.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"skill_name": {
				Type:        schema.String,
				Desc:        "The name of the skill (e.g., 'csv-data-analysis')",
				Required:    true,
			},
			"script_file_name": {
				Type:        schema.String,
				Desc:        "The script filename in the skill's scripts directory (e.g., 'csv_analyzer.py')",
				Required:    true,
			},
			"args": {
				Type:        schema.Object,
				Desc:        "Arguments to pass to the script as a JSON object",
				Required:    false,
			},
			"output_dir": {
				Type:        schema.String,
				Desc:        "Output directory for script execution (optional)",
				Required:    false,
			},
		}),
	}, nil
}

func (t *ExecuteSkillScriptFileTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var scriptInput ExecuteSkillScriptInput
	if err := json.Unmarshal([]byte(input), &scriptInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if scriptInput.SkillName == "" {
		return &ValidationResult{Valid: false, Message: "skill_name is required", ErrorCode: 2}
	}
	if scriptInput.ScriptFileName == "" {
		return &ValidationResult{Valid: false, Message: "script_file_name is required", ErrorCode: 2}
	}
	return &ValidationResult{Valid: true}
}

func (t *ExecuteSkillScriptFileTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var scriptInput ExecuteSkillScriptInput
	if err := json.Unmarshal([]byte(input), &scriptInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	// Resolve skills directory
	skillsDir := t.skillsDir
	if skillsDir == "" {
		skillsDir = os.Getenv("XQL_SKILLS_DIR")
	}
	if skillsDir == "" {
		// Fallback to default xqldir location
		home, _ := os.UserHomeDir()
		skillsDir = filepath.Join(home, ".xiaoqinglong", "skills")
	}

	// Translate sandbox mount paths to host paths
	// /mnt/uploads/ -> actual uploads dir (set by XQL_UPLOADS_DIR env or default)
	if scriptInput.Args != nil {
		if inputFile, ok := scriptInput.Args["input_file"].(string); ok {
			if strings.HasPrefix(inputFile, "/mnt/uploads/") {
				uploadsDir := os.Getenv("XQL_UPLOADS_DIR")
				if uploadsDir == "" {
					home, _ := os.UserHomeDir()
					uploadsDir = filepath.Join(home, ".xiaoqinglong", "data", "uploads")
				}
				// Replace /mnt/uploads/ with actual uploads dir
				scriptInput.Args["input_file"] = strings.Replace(inputFile, "/mnt/uploads/", uploadsDir+"/", 1)
			}
		}
	}

	// Build script path
	scriptFileName := scriptInput.ScriptFileName
	scriptFileName = strings.TrimPrefix(scriptFileName, "scripts/")
	scriptFileName = strings.TrimPrefix(scriptFileName, "scripts\\")

	scriptPath := filepath.Join(skillsDir, scriptInput.SkillName, "scripts", scriptFileName)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		result := ExecuteSkillScriptOutput{
			Chunks: []SkillScriptChunk{
				{OutputType: "text", Content: fmt.Sprintf("Script file not found: %s", scriptPath)},
			},
		}
		data, _ := json.Marshal(result)
		return string(data), nil
	}

	// Read script content
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		result := ExecuteSkillScriptOutput{
			Chunks: []SkillScriptChunk{
				{OutputType: "text", Content: fmt.Sprintf("Error reading script: %v", err)},
			},
		}
		data, _ := json.Marshal(result)
		return string(data), nil
	}

	// Determine working directory
	workDir := scriptInput.OutputDir
	if workDir == "" {
		workDir = filepath.Dir(scriptPath)
	}
	os.MkdirAll(workDir, 0755)

	// Determine script language and wrap code
	code := string(scriptContent)
	ext := strings.ToLower(filepath.Ext(scriptPath))
	var execCode string

	if ext == ".py" {
		// Wrap Python script with sys.argv setup
		argsRepr, _ := json.Marshal(scriptInput.Args)
		execCode = fmt.Sprintf(`import sys
import json

sys.argv = ["script", json.dumps(%s)]
__name__ = "__main__"

%s`, string(argsRepr), code)
	} else {
		// For non-Python scripts, execute directly
		execCode = code
	}

	// Execute the script
	var stdout, stderr bytes.Buffer
	var exitCode int

	if ext == ".py" {
		// Write wrapper to temp file
		tmpFile, err := os.CreateTemp(filepath.Dir(scriptPath), "_skill_run_*.py")
		if err != nil {
			result := ExecuteSkillScriptOutput{
				Chunks: []SkillScriptChunk{
					{OutputType: "text", Content: fmt.Sprintf("Failed to create temp file: %v", err)},
				},
			}
			data, _ := json.Marshal(result)
		return string(data), nil
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(execCode); err != nil {
			result := ExecuteSkillScriptOutput{
				Chunks: []SkillScriptChunk{
					{OutputType: "text", Content: fmt.Sprintf("Failed to write temp file: %v", err)},
				},
			}
			data, _ := json.Marshal(result)
		return string(data), nil
		}
		tmpFile.Close()

		cmdCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()

		cmd := exec.CommandContext(cmdCtx, "python3", tmpFile.Name())
		cmd.Dir = workDir
		cmd.Env = os.Environ()
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
	} else {
		// Bash script
		cmdCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()

		cmd := exec.CommandContext(cmdCtx, "bash", "-c", execCode)
		cmd.Dir = workDir
		cmd.Env = os.Environ()
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
	}

	outputText := stdout.String()
	stderrText := stderr.String()

	chunks := []SkillScriptChunk{}

	// Try to parse output as JSON first
	if outputText != "" {
		trimmed := strings.TrimSpace(outputText)
		var parsed map[string]any
		if json.Unmarshal([]byte(trimmed), &parsed) == nil {
			if chunkList, ok := parsed["chunks"].([]any); ok {
				for _, c := range chunkList {
					if chunkMap, ok := c.(map[string]any); ok {
						chunkContent := getString(chunkMap, "content", "")
						chunk := SkillScriptChunk{
							OutputType: getString(chunkMap, "output_type", "text"),
							Content:    chunkContent,
						}
						chunks = append(chunks, chunk)
					}
				}
			}
			if len(chunks) == 0 {
				chunks = append(chunks, SkillScriptChunk{OutputType: "text", Content: trimmed})
			}
		} else {
			// For non-JSON output, extract markers from raw output and clean content
			markers := extractScriptMarkers(outputText)
			cleanContent := outputText
			for key, value := range markers {
				// Remove marker blocks from the displayed content
				markerPattern := fmt.Sprintf(`###%s_START###.*?###%s_END###`, key, key)
				re := regexp.MustCompile(markerPattern)
				cleanContent = re.ReplaceAllString(cleanContent, "")
				// Add marker data as separate chunk with key name
				if value != "" {
					chunks = append(chunks, SkillScriptChunk{
						OutputType: "data",
						Content:    value,
						Key:        key,
					})
				}
			}
			cleanContent = strings.TrimSpace(cleanContent)
			if cleanContent != "" {
				chunks = append(chunks, SkillScriptChunk{OutputType: "text", Content: cleanContent})
			}
		}
	}

	// Extract markers from chunk contents (these are properly unescaped after JSON parsing)
	// and store them for html_interpreter auto-injection
	if scriptInput.SkillName != "" {
		allMarkers := make(map[string]string)
		for _, chunk := range chunks {
			if chunk.Content != "" {
				chunkMarkers := extractScriptMarkers(chunk.Content)
				for k, v := range chunkMarkers {
					allMarkers[k] = v
				}
			}
		}
		if len(allMarkers) > 0 {
			SetSkillMarkers(scriptInput.SkillName, allMarkers)
		}
	}

	// Add stderr as error if present
	if stderrText != "" && exitCode != 0 {
		chunks = append(chunks, SkillScriptChunk{OutputType: "text", Content: fmt.Sprintf("[ERROR] %s", stderrText)})
	}

	// Add exit code if non-zero
	if exitCode != 0 {
		chunks = append(chunks, SkillScriptChunk{OutputType: "text", Content: fmt.Sprintf("Exit code: %d", exitCode)})
	}

	if len(chunks) == 0 {
		chunks = append(chunks, SkillScriptChunk{OutputType: "text", Content: "Script executed successfully (no output)"})
	}

	result := ExecuteSkillScriptOutput{Chunks: chunks}
	data, _ := json.Marshal(result)
	return string(data), nil
}

func getString(m map[string]any, key, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
}

// imageExtRegex matches image file extensions
var imageExtRegex = regexp.MustCompile(`\.(png|jpg|jpeg|gif|svg|webp)$`)

// extractScriptMarkers extracts ###KEY_START###...###KEY_END### blocks from output
func extractScriptMarkers(output string) map[string]string {
	markers := make(map[string]string)
	// Find all KEY names from START markers
	startRe := regexp.MustCompile(`###(\w+)_START###`)
	startMatches := startRe.FindAllStringSubmatchIndex(output, -1)
	for _, match := range startMatches {
		if len(match) >= 4 {
			key := output[match[2]:match[3]]
			// match[1] is the position right after the full ###KEY_START### match ends
			// The content starts after the newline following the START marker
			startPos := match[1]
			endMarker := "###" + key + "_END###"
			// endPos is relative to startPos, so absolute position is startPos + endPos
			endPosRel := strings.Index(output[startPos:], endMarker)
			if endPosRel >= 0 {
				endMarkerAbsPos := startPos + endPosRel
				// Content starts after the newline following START marker
				contentStart := startPos + 1
				// Content ends at the start of the END marker
				if contentStart < endMarkerAbsPos {
					markers[key] = output[contentStart:endMarkerAbsPos]
				}
			}
		}
	}
	return markers
}