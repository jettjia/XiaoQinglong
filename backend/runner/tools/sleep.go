package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ========== SleepTool ==========

// SleepInput for sleep tool
type SleepInput struct {
	Seconds float64 `json:"seconds"` // Number of seconds to sleep
}

// SleepOutput for sleep result
type SleepOutput struct {
	Slept float64 `json:"slept"` // Actual seconds slept
}

// SleepTool waits for a specified duration
type SleepTool struct{}

func NewSleepTool() *SleepTool {
	return &SleepTool{}
}

func init() {
	GlobalRegistry.Register(ToolMeta{
		Name:           "Sleep",
		Desc:           "Wait for a specified duration. Use this to add delays between operations.",
		IsReadOnly:     true, // 只读，不修改任何东西
		MaxResultChars: 100,
		DefaultRisk:    "low",
		Creator: func(basePath string) interface{} {
			return NewSleepTool()
		},
	})
}

func (t *SleepTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "Sleep",
		Desc: "Wait for a specified duration. Use this to add delays between operations.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"seconds": {
				Type:        schema.Number,
				Desc:        "Number of seconds to sleep (supports fractional values like 0.5)",
				Required:    true,
			},
		}),
	}, nil
}

func (t *SleepTool) ValidateInput(ctx context.Context, input string) *ValidationResult {
	var sleepInput SleepInput
	if err := json.Unmarshal([]byte(input), &sleepInput); err != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("invalid JSON: %v", err), ErrorCode: 1}
	}
	if sleepInput.Seconds <= 0 {
		return &ValidationResult{Valid: false, Message: "seconds must be positive", ErrorCode: 2}
	}
	return &ValidationResult{Valid: true}
}

func (t *SleepTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var sleepInput SleepInput
	if err := json.Unmarshal([]byte(input), &sleepInput); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	// Create a timer
	timer := time.NewTimer(time.Duration(float64(sleepInput.Seconds) * float64(time.Second)))

	select {
	case <-ctx.Done():
		timer.Stop()
		return "", ctx.Err()
	case <-timer.C:
		output := SleepOutput{Slept: sleepInput.Seconds}
		result, _ := json.Marshal(output)
		return string(result), nil
	}
}
