package prompt

import (
	"strings"

	"github.com/jettjia/XiaoQinglong/runner/types"
)

// ========== Prompt Builder ==========

// PromptBuilder builds structured system prompts
type PromptBuilder struct {
	sections    []PromptSection
	staticCache string // Cache for static sections
}

// NewPromptBuilder creates a new prompt builder
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{
		sections: make([]PromptSection, 0),
	}
}

// AddStaticSection adds a static (non-changing) section
func (b *PromptBuilder) AddStaticSection(sectionType SectionType, content string) *PromptBuilder {
	if content == "" {
		return b
	}
	b.sections = append(b.sections, PromptSection{
		Type:    sectionType,
		Content: content,
		Dynamic: false,
	})
	return b
}

// AddDynamicSection adds a dynamic section that should be recalculated
func (b *PromptBuilder) AddDynamicSection(sectionType SectionType, content string) *PromptBuilder {
	if content == "" {
		return b
	}
	b.sections = append(b.sections, PromptSection{
		Type:    sectionType,
		Content: content,
		Dynamic: true,
	})
	return b
}

// AddCustomSection adds a custom section with the given type and content
func (b *PromptBuilder) AddCustomSection(name string, content string) *PromptBuilder {
	if content == "" {
		return b
	}
	b.sections = append(b.sections, PromptSection{
		Type:    SectionType(name),
		Content: content,
		Dynamic: false,
	})
	return b
}

// Build builds the final prompt string
func (b *PromptBuilder) Build() string {
	if len(b.sections) == 0 {
		return ""
	}

	var parts []string
	for _, section := range b.sections {
		if section.Content != "" {
			parts = append(parts, section.Content)
		}
	}

	return strings.Join(parts, "\n\n")
}

// BuildWithCustomPrompt builds prompt with custom system prompt prepended
func (b *PromptBuilder) BuildWithCustomPrompt(customPrompt string) string {
	var parts []string

	// Prepend custom system prompt if provided
	if customPrompt != "" {
		parts = append(parts, customPrompt)
	}

	// Add all sections
	for _, section := range b.sections {
		if section.Content != "" {
			parts = append(parts, section.Content)
		}
	}

	return strings.Join(parts, "\n\n")
}

// GetStaticSections returns only the static sections joined together
func (b *PromptBuilder) GetStaticSections() string {
	var parts []string
	for _, section := range b.sections {
		if !section.Dynamic && section.Content != "" {
			parts = append(parts, section.Content)
		}
	}
	return strings.Join(parts, "\n\n")
}

// GetDynamicSections returns only the dynamic sections joined together
func (b *PromptBuilder) GetDynamicSections() string {
	var parts []string
	for _, section := range b.sections {
		if section.Dynamic && section.Content != "" {
			parts = append(parts, section.Content)
		}
	}
	return strings.Join(parts, "\n\n")
}

// Sections returns all sections
func (b *PromptBuilder) Sections() []PromptSection {
	return b.sections
}

// Clear removes all sections
func (b *PromptBuilder) Clear() *PromptBuilder {
	b.sections = make([]PromptSection, 0)
	b.staticCache = ""
	return b
}

// ========== Default Prompt Builder Factory ==========

// BuildDefaultPrompt builds the default system prompt using the prompt builder
func BuildDefaultPrompt(req *types.RunRequest, enabledTools []string) string {
	builder := NewPromptBuilder()

	// Static sections (these don't change between requests)
	builder.AddStaticSection(IntroSection, GetIntroSection())
	builder.AddStaticSection(SystemSection, GetSystemSection())
	builder.AddStaticSection(DoingTasksSection, GetDoingTasksSection())
	builder.AddStaticSection(ActionsSection, GetActionsSection())
	builder.AddStaticSection(UsingYourToolsSection, GetUsingYourToolsSection(enabledTools))
	builder.AddStaticSection(OutputEfficiencySection, GetOutputEfficiencySection())
	builder.AddStaticSection(ToneAndStyleSection, GetToneAndStyleSection())

	// Dynamic sections (these are recalculated per request)
	builder.AddDynamicSection(SkillsSection, GetSkillsSection(req.Skills))
	builder.AddDynamicSection(McpSection, GetMcpSection(req.MCPs))
	builder.AddDynamicSection(ContextSection, GetContextSection(req.Context))
	builder.AddDynamicSection(FilesSection, GetFilesSection(req.Files))
	builder.AddDynamicSection(A2AAgentsSection, GetA2AAgentsSection(req.A2A))
	builder.AddDynamicSection(InternalAgentsSection, GetInternalAgentsSection(req.InternalAgents))
	builder.AddDynamicSection(ResponseSchemaSection, GetResponseSchemaSection(req.Options.ResponseSchema))

	// Build with custom prompt prepended
	return builder.BuildWithCustomPrompt(req.Prompt)
}

// BuildSimplePrompt builds a simple prompt without the full section structure
// This is useful when you just need a basic prompt quickly
func BuildSimplePrompt(customPrompt string) string {
	builder := NewPromptBuilder()

	builder.AddStaticSection(IntroSection, GetIntroSection())
	builder.AddStaticSection(SystemSection, GetSystemSection())
	builder.AddStaticSection(OutputEfficiencySection, GetOutputEfficiencySection())
	builder.AddStaticSection(ToneAndStyleSection, GetToneAndStyleSection())

	return builder.BuildWithCustomPrompt(customPrompt)
}

// BuildStaticPrompt builds only the static sections (Intro, System, DoingTasks, Actions, UsingYourTools, OutputEfficiency, ToneAndStyle)
// These can be cached per agent + tools combination
func BuildStaticPrompt(enabledTools []string) string {
	builder := NewPromptBuilder()

	builder.AddStaticSection(IntroSection, GetIntroSection())
	builder.AddStaticSection(SystemSection, GetSystemSection())
	builder.AddStaticSection(DoingTasksSection, GetDoingTasksSection())
	builder.AddStaticSection(ActionsSection, GetActionsSection())
	builder.AddStaticSection(UsingYourToolsSection, GetUsingYourToolsSection(enabledTools))
	builder.AddStaticSection(OutputEfficiencySection, GetOutputEfficiencySection())
	builder.AddStaticSection(ToneAndStyleSection, GetToneAndStyleSection())

	return builder.Build()
}

// BuildDynamicPrompt builds only the dynamic sections (Skills, MCPs, Context, Files, A2A, InternalAgents, ResponseSchema)
// These are always recalculated per request
func BuildDynamicPrompt(req *types.RunRequest) string {
	builder := NewPromptBuilder()

	builder.AddDynamicSection(SkillsSection, GetSkillsSection(req.Skills))
	builder.AddDynamicSection(McpSection, GetMcpSection(req.MCPs))
	builder.AddDynamicSection(ContextSection, GetContextSection(req.Context))
	builder.AddDynamicSection(FilesSection, GetFilesSection(req.Files))
	builder.AddDynamicSection(A2AAgentsSection, GetA2AAgentsSection(req.A2A))
	builder.AddDynamicSection(InternalAgentsSection, GetInternalAgentsSection(req.InternalAgents))
	builder.AddDynamicSection(ResponseSchemaSection, GetResponseSchemaSection(req.Options.ResponseSchema))

	return builder.Build()
}
