package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== Plan Mode Types ==========

// PlanModeState tracks plan mode state
type PlanModeState struct {
	mu         sync.RWMutex
	inPlanMode bool
	phase      string // "planning", "implementation", "review"
}

// GlobalPlanModeState is the global plan mode state
var GlobalPlanModeState = &PlanModeState{}

func (s *PlanModeState) Enter() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inPlanMode {
		return false
	}
	s.inPlanMode = true
	s.phase = "planning"
	return true
}

func (s *PlanModeState) Exit() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.inPlanMode {
		return false
	}
	s.inPlanMode = false
	s.phase = ""
	return true
}

func (s *PlanModeState) IsInPlanMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inPlanMode
}

func (s *PlanModeState) SetPhase(phase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = phase
}

func (s *PlanModeState) GetPhase() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.phase
}

// ========== EnterPlanModeTool ==========

// EnterPlanModeInput for enter plan mode tool
type EnterPlanModeInput struct {
	Reason string `json:"reason,omitempty"` // Reason for entering plan mode
}

// EnterPlanModeOutput for enter plan mode result
type EnterPlanModeOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// EnterPlanModeTool enters plan mode for implementation planning
type EnterPlanModeTool struct {
	state *PlanModeState
}

func NewEnterPlanModeTool() *EnterPlanModeTool {
	return &EnterPlanModeTool{state: GlobalPlanModeState}
}

func (t *EnterPlanModeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "EnterPlanMode",
		Desc: "Enter plan mode for implementation planning before coding. Use this when you need to design an approach or get user approval before proceeding.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"reason": {
				Type:        schema.String,
				Desc:        "Reason for entering plan mode",
				Required:    false,
			},
		}),
	}, nil
}

func (t *EnterPlanModeTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	return &ValidationResult{Valid: true}
}

func (t *EnterPlanModeTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var planInput EnterPlanModeInput
	if input != "" {
		json.Unmarshal([]byte(input), &planInput)
	}

	reason := planInput.Reason
	if reason == "" {
		reason = "Planning implementation approach"
	}

	if !t.state.Enter() {
		output := EnterPlanModeOutput{
			Success: false,
			Message: "Already in plan mode",
		}
		result, _ := json.Marshal(output)
		return string(result), nil
	}

	output := EnterPlanModeOutput{
		Success: true,
		Message: fmt.Sprintf("Entered plan mode. %s", reason),
	}
	result, _ := json.Marshal(output)
	return string(result), nil
}

// ========== ExitPlanModeTool ==========

// ExitPlanModeInput for exit plan mode tool
type ExitPlanModeInput struct {
	Approved   bool   `json:"approved"`    // Whether the plan is approved
	Feedback   string `json:"feedback,omitempty"` // User feedback on the plan
}

// ExitPlanModeOutput for exit plan mode result
type ExitPlanModeOutput struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	CanProceed bool   `json:"can_proceed"`
}

// ExitPlanModeTool exits plan mode with approval
type ExitPlanModeTool struct {
	state *PlanModeState
}

func NewExitPlanModeTool() *ExitPlanModeTool {
	return &ExitPlanModeTool{state: GlobalPlanModeState}
}

func (t *ExitPlanModeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "ExitPlanMode",
		Desc: "Exit plan mode with user approval. Call this when the plan is ready for implementation.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"approved": {
				Type:        schema.Boolean,
				Desc:        "Whether the plan is approved to proceed",
				Required:    true,
			},
			"feedback": {
				Type:        schema.String,
				Desc:        "User feedback on the plan",
				Required:    false,
			},
		}),
	}, nil
}

func (t *ExitPlanModeTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var exitInput ExitPlanModeInput
	if err := json.Unmarshal([]byte(input), &exitInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	return &ValidationResult{Valid: true}
}

func (t *ExitPlanModeTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var exitInput ExitPlanModeInput
	if err := json.Unmarshal([]byte(input), &exitInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if !t.state.Exit() {
		output := ExitPlanModeOutput{
			Success:    false,
			Message:    "Not in plan mode",
			CanProceed: false,
		}
		result, _ := json.Marshal(output)
		return string(result), nil
	}

	message := "Plan approved, ready to proceed"
	if !exitInput.Approved {
		message = "Plan not approved"
	}

	output := ExitPlanModeOutput{
		Success:    true,
		Message:    message,
		CanProceed: exitInput.Approved,
	}
	result, _ := json.Marshal(output)
	return string(result), nil
}
