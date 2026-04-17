package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// HtmlInterpreterInput for html_interpreter tool
type HtmlInterpreterInput struct {
	// TemplatePath is the path to an HTML template file (relative to skills dir or absolute)
	// Example: "csv-data-analysis/templates/report_template.html"
	TemplatePath string `json:"template_path,omitempty"`
	// Data is a map of placeholder keys to values for template replacement
	Data map[string]any `json:"data,omitempty"`
	// Html is direct HTML content (used instead of template_path)
	Html string `json:"html,omitempty"`
	// Title is the page title (used for direct HTML mode)
	Title string `json:"title,omitempty"`
	// OutputPath is the path to save the rendered HTML file (optional)
	// Example: "report_order_analysis.html" or "/reports/report.html"
	// If provided, the HTML will be saved to this path and made accessible via /reports/ URL
	OutputPath string `json:"output_path,omitempty"`
	// SessionID is the session ID for session-specific report storage (optional)
	// If provided, the report will be saved to ~/.xiaoqinglong/data/reports/{session_id}/{output_path}
	SessionID string `json:"session_id,omitempty"`
}

// HtmlInterpreterOutput for html_interpreter result
type HtmlInterpreterOutput struct {
	Type    string `json:"type"`     // "html_report"
	Url     string `json:"url"`      // URL to access the saved file, e.g. "/reports/xxx.html"
	Title   string `json:"title"`    // Page title
	Html    string `json:"html"`     // Rendered HTML content (when not saved)
	Saved   bool   `json:"saved"`    // Whether the file was saved
	Message string `json:"message"`  // Status message
}

// HtmlInterpreterTool renders HTML content or templates
type HtmlInterpreterTool struct {
	skillsDir string
}

func NewHtmlInterpreterTool(skillsDir string) *HtmlInterpreterTool {
	return &HtmlInterpreterTool{
		skillsDir: skillsDir,
	}
}

func init() {
	GlobalRegistry.Register(ToolMeta{
		Name:           "html_interpreter",
		Desc:           "Render HTML content as a web report. Supports template-based rendering with placeholder replacement.",
		IsReadOnly:     true,
		MaxResultChars: 1000000,
		DefaultRisk:    "low",
		Creator: func(basePath string) interface{} {
			return NewHtmlInterpreterTool("")
		},
	})
}

func (t *HtmlInterpreterTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "html_interpreter",
		Desc: "Render HTML content as a web report. Supports template-based rendering with {{PLACEHOLDER}} replacement or direct HTML input. Use output_path to save the report to a file accessible via /reports/ URL.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"template_path": {
				Type:        schema.String,
				Desc:        "Path to HTML template (relative to skills dir or absolute). Example: csv-data-analysis/templates/report_template.html",
				Required:    false,
			},
			"data": {
				Type:        schema.Object,
				Desc:        "Map of placeholder keys to values for template replacement. Example: {\"REPORT_TITLE\": \"My Report\", \"LANG\": \"en\"}",
				Required:    false,
			},
			"html": {
				Type:        schema.String,
				Desc:        "Direct HTML content (used instead of template_path)",
				Required:    false,
			},
			"title": {
				Type:        schema.String,
				Desc:        "Page title for direct HTML mode",
				Required:    false,
			},
			"output_path": {
				Type:        schema.String,
				Desc:        "Path to save the rendered HTML file (relative to reports dir). Example: report_order_analysis.html. The file will be accessible via /reports/{session_id}/{output_path}",
				Required:    false,
			},
			"session_id": {
				Type:        schema.String,
				Desc:        "Session ID for session-specific report storage. Reports will be saved to ~/.xiaoqinglong/data/reports/{session_id}/{output_path}",
				Required:    false,
			},
		}),
	}, nil
}

func (t *HtmlInterpreterTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var htmlInput HtmlInterpreterInput
	if err := json.Unmarshal([]byte(input), &htmlInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if htmlInput.TemplatePath == "" && htmlInput.Html == "" {
		return &ValidationResult{Valid: false, Message: "either template_path or html is required", ErrorCode: 2}
	}
	return &ValidationResult{Valid: true}
}

func (t *HtmlInterpreterTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var htmlInput HtmlInterpreterInput
	if err := json.Unmarshal([]byte(input), &htmlInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	var htmlContent string
	var title string

	if htmlInput.Html != "" {
		// Direct HTML mode
		htmlContent = htmlInput.Html
		title = htmlInput.Title
		if title == "" {
			title = "Report"
		}
	} else {
		// Template mode
		templatePath := htmlInput.TemplatePath
		if templatePath == "" {
			result := HtmlInterpreterOutput{
				Message: "template_path is required when html is not provided",
			}
			data, _ := json.Marshal(result)
			return string(data), nil
		}

		// Resolve template path
		fullPath := templatePath
		if !filepath.IsAbs(fullPath) {
			// Try skills dir first
			skillsDir := t.skillsDir
			if skillsDir == "" {
				skillsDir = os.Getenv("XQL_SKILLS_DIR")
			}
			if skillsDir == "" {
				home, _ := os.UserHomeDir()
				skillsDir = filepath.Join(home, ".xiaoqinglong", "skills")
			}

			// Try skills/{skill_name}/templates/{path}
			// First extract skill name from template_path if it's like "csv-data-analysis/templates/..."
			parts := strings.Split(templatePath, "/")
			if len(parts) >= 2 && parts[0] != "" {
				// Check if first part looks like a skill name (contains letters)
				if strings.Contains(parts[0], "-") || strings.Contains(parts[0], "_") {
					skillPath := filepath.Join(skillsDir, templatePath)
					if _, err := os.Stat(skillPath); err == nil {
						fullPath = skillPath
					}
				}
			}

			// If not found, try as absolute path or relative to current dir
			if fullPath == templatePath {
				if _, err := os.Stat(fullPath); os.IsNotExist(err) {
					result := HtmlInterpreterOutput{
						Message: fmt.Sprintf("Template file not found: %s", templatePath),
					}
					data, _ := json.Marshal(result)
					return string(data), nil
				}
			}
		}

		// Read template file
		content, err := os.ReadFile(fullPath)
		if err != nil {
			result := HtmlInterpreterOutput{
				Message: fmt.Sprintf("Error reading template: %v", err),
			}
			data, _ := json.Marshal(result)
			return string(data), nil
		}
		htmlContent = string(content)

		// Auto-inject skill markers (CHART_DATA_JSON etc.) if not already in Data
		// Extract skill name from template path (e.g., "csv-data-analysis/templates/..." -> "csv-data-analysis")
		skillName := extractSkillNameFromPath(htmlInput.TemplatePath)
		markers := make(map[string]any)
		if skillName != "" {
			if storedMarkers := GetSkillMarkers(skillName); storedMarkers != nil {
				for k, v := range storedMarkers {
					markers[k] = v
				}
				// Clear markers after retrieval to avoid stale data
				ClearSkillMarkers(skillName)
			}
		}

		// Build final data: skill markers + htmlInput.Data (Data takes precedence)
		finalData := markers
		if htmlInput.Data != nil {
			for k, v := range htmlInput.Data {
				finalData[k] = v
			}
		}

		// Replace placeholders
		if len(finalData) > 0 {
			htmlContent = replacePlaceholders(htmlContent, finalData)
		}

		// Extract title from template if available
		title = extractTitle(htmlContent)
		if title == "" {
			title = "Report"
		}
	}

	// Handle saving: auto-generate output_path if not provided
	var savedUrl string
	sessionID := htmlInput.SessionID
	if sessionID == "" {
		sessionID = os.Getenv("XQL_SESSION_ID")
	}
	outputPath := htmlInput.OutputPath
	if outputPath == "" {
		// Auto-generate a filename based on title and timestamp
		timestamp := time.Now().Format("20060102_150405")
		safeTitle := regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(title, "_")
		if len(safeTitle) > 20 {
			safeTitle = safeTitle[:20]
		}
		outputPath = fmt.Sprintf("report_%s_%s.html", safeTitle, timestamp)
	}
	savedUrl = saveHtmlReport(outputPath, htmlContent, sessionID)

	result := HtmlInterpreterOutput{
		Type:    "html_report",
		Url:     savedUrl,
		Title:   title,
		Html:    htmlContent,
		Saved:   savedUrl != "",
		Message: "HTML rendered successfully",
	}
	data, _ := json.Marshal(result)
	return string(data), nil
}

// replacePlaceholders replaces {{KEY}} placeholders with values from data map
func replacePlaceholders(content string, data map[string]any) string {
	// Pattern to match {{PLACEHOLDER}} with word characters
	result := content

	// Replace each placeholder
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) == 2 {
			key := match[1]
			if value, ok := data[key]; ok {
				result = strings.ReplaceAll(result, "{{"+key+"}}", fmt.Sprintf("%v", value))
			}
		}
	}

	return result
}

// extractTitle extracts <title> content from HTML
func extractTitle(html string) string {
	start := strings.Index(html, "<title>")
	if start == -1 {
		start = strings.Index(html, "<TITLE>")
	}
	if start == -1 {
		return ""
	}
	start += 7
	end := strings.Index(html[start:], "</title>")
	if end == -1 {
		end = strings.Index(html[start:], "</TITLE>")
	}
	if end == -1 {
		return ""
	}
	return html[start : start+end]
}

// extractSkillNameFromPath extracts the skill name from a template path
// e.g., "csv-data-analysis/templates/report_template.html" -> "csv-data-analysis"
func extractSkillNameFromPath(templatePath string) string {
	parts := strings.Split(templatePath, "/")
	if len(parts) >= 1 {
		// First part could be the skill name (contains dash or underscore)
		if strings.Contains(parts[0], "-") || strings.Contains(parts[0], "_") {
			return parts[0]
		}
	}
	return ""
}

// saveHtmlReport saves HTML content to the reports directory
// Returns the URL path to access the file, or empty string on error
func saveHtmlReport(outputPath string, content string, sessionID string) string {
	// Get reports directory
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	reportsDir := filepath.Join(home, ".xiaoqinglong", "data", "reports")

	// If sessionID is provided, save under session-specific subdirectory
	if sessionID != "" {
		reportsDir = filepath.Join(reportsDir, sessionID)
	}

	// Ensure directory exists
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return ""
	}

	// Clean the path to prevent directory traversal
	outputPath = filepath.Clean(outputPath)
	if filepath.IsAbs(outputPath) {
		// If absolute path, use just the filename
		outputPath = filepath.Base(outputPath)
	}
	// Remove any leading slashes or dots
	outputPath = strings.TrimPrefix(outputPath, "/")
	outputPath = strings.TrimPrefix(outputPath, ".")

	// Build full file path
	fullPath := filepath.Join(reportsDir, outputPath)

	// Write the file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return ""
	}

	// Return URL path
	if sessionID != "" {
		return "/reports/" + sessionID + "/" + outputPath
	}
	return "/reports/" + outputPath
}