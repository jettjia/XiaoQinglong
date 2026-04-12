package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== AskUserQuestionTool ==========

// QuestionOption represents a single choice option
type QuestionOption struct {
	Label       string `json:"label"`                // Display label
	Description string `json:"description,omitempty"` // Option description
}

// QuestionInput for ask user question tool
type QuestionInput struct {
	Question  string           `json:"question"`            // The question to ask
	Options   []QuestionOption `json:"options"`            // Answer options
	Header    string           `json:"header,omitempty"`  // Short header for the question
	MultiSelect bool           `json:"multi_select"`      // Allow multiple selections
}

// QuestionOutput for ask user question result
type QuestionOutput struct {
	Answer    string   `json:"answer"`     // Selected option label
	Answers   []string `json:"answers"`    // Multiple selected labels (if multi_select)
	Success   bool     `json:"success"`   // Whether an answer was received
}

// AskUserQuestionTool asks the user multiple choice questions
type AskUserQuestionTool struct{}

func NewAskUserQuestionTool() *AskUserQuestionTool {
	return &AskUserQuestionTool{}
}

func init() {
	GlobalRegistry.Register(ToolMeta{
		Name:           "AskUserQuestion",
		Desc:           "Ask the user multiple choice questions to gather information, clarify ambiguity, or understand preferences.",
		IsReadOnly:     false,
		MaxResultChars: 1000,
		DefaultRisk:    "low",
		Creator: func(basePath string) interface{} {
			return NewAskUserQuestionTool()
		},
	})
}

func (t *AskUserQuestionTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "AskUserQuestion",
		Desc: "Ask the user multiple choice questions to gather information, clarify ambiguity, or understand preferences.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"question": {
				Type:        schema.String,
				Desc:        "The question to ask the user",
				Required:    true,
			},
			"options": {
				Type:        schema.Array,
				Desc:        "Answer options (each with label and optional description)",
				Required:    true,
			},
			"header": {
				Type:        schema.String,
				Desc:        "Short header/chip for the question (max 12 chars)",
				Required:    false,
			},
			"multi_select": {
				Type:        schema.Boolean,
				Desc:        "Allow multiple selections (default false)",
				Required:    false,
			},
		}),
	}, nil
}

func (t *AskUserQuestionTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var questionInput QuestionInput
	if err := json.Unmarshal([]byte(input), &questionInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if questionInput.Question == "" {
		return &ValidationResult{Valid: false, Message: "question is required", ErrorCode: 2}
	}
	if len(questionInput.Options) < 2 {
		return &ValidationResult{Valid: false, Message: "at least 2 options are required", ErrorCode: 3}
	}
	if len(questionInput.Options) > 4 {
		return &ValidationResult{Valid: false, Message: "maximum 4 options allowed", ErrorCode: 4}
	}
	return &ValidationResult{Valid: true}
}

func (t *AskUserQuestionTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var questionInput QuestionInput
	if err := json.Unmarshal([]byte(input), &questionInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	// In a CLI context, this would present the question to the user
	// and wait for their response. For now, we return a structure
	// that indicates the question was asked.

	// Build options description
	optionsDesc := make([]string, 0)
	for i, opt := range questionInput.Options {
		desc := fmt.Sprintf("%d. %s", i+1, opt.Label)
		if opt.Description != "" {
			desc += fmt.Sprintf(" - %s", opt.Description)
		}
		optionsDesc = append(optionsDesc, desc)
	}

	// Return the question details for the system to handle
	// For now, return the formatted question
	// In a real implementation, this would block until user responds
	type QuestionForUI struct {
		Question    string           `json:"question"`
		Options     []QuestionOption `json:"options"`
		Header      string           `json:"header,omitempty"`
		MultiSelect bool             `json:"multi_select"`
	}

	q := QuestionForUI{
		Question:    questionInput.Question,
		Options:     questionInput.Options,
		Header:      questionInput.Header,
		MultiSelect: questionInput.MultiSelect,
	}

	result, _ := json.Marshal(q)
	return string(result), nil
}
