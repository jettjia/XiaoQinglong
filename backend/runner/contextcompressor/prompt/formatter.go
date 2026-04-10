package prompt

import (
	"regexp"
	"strings"
)

// PartialCompactDirection 部分压缩方向
type PartialCompactDirection string

const (
	PartialCompactDirectionFrom  PartialCompactDirection = "from"  // 从早期保留点到最近
	PartialCompactDirectionUpTo PartialCompactDirection = "up_to" // 从开始到指定点
)

// GetCompactPrompt 获取完整压缩提示词
func GetCompactPrompt(customInstructions string) string {
	prompt := NO_TOOLS_PREAMBLE + BASE_COMPACT_PROMPT

	if customInstructions != "" && strings.TrimSpace(customInstructions) != "" {
		prompt += "\n\nAdditional Instructions:\n" + customInstructions
	}

	prompt += NO_TOOLS_TRAILER
	return prompt
}

// GetPartialCompactPrompt 获取部分压缩提示词
func GetPartialCompactPrompt(customInstructions string, direction PartialCompactDirection) string {
	var template string
	if direction == PartialCompactDirectionUpTo {
		template = PARTIAL_COMPACT_UP_TO_PROMPT
	} else {
		template = PARTIAL_COMPACT_PROMPT
	}

	prompt := NO_TOOLS_PREAMBLE + template

	if customInstructions != "" && strings.TrimSpace(customInstructions) != "" {
		prompt += "\n\nAdditional Instructions:\n" + customInstructions
	}

	prompt += NO_TOOLS_TRAILER
	return prompt
}

// FormatCompactSummary 格式化 LLM 输出的摘要
// 参考: src/services/compact/prompt.ts:311-335
func FormatCompactSummary(summary string) string {
	formatted := summary

	// Strip analysis section — it's a drafting scratchpad that improves summary
	// quality but has no informational value once the summary is written.
	analysisRegex := regexp.MustCompile(`<analysis>[\s\S]*?</analysis>`)
	formatted = analysisRegex.ReplaceAllString(formatted, "")

	// Extract and format summary section
	summaryRegex := regexp.MustCompile(`<summary>([\s\S]*?)</summary>`)
	matches := summaryRegex.FindStringSubmatch(formatted)
	if len(matches) > 1 {
		content := strings.TrimSpace(matches[1])
		formatted = summaryRegex.ReplaceAllString(formatted, "Summary:\n"+content)
	}

	// Clean up extra whitespace between sections
	multiNewline := regexp.MustCompile(`\n\n+`)
	formatted = multiNewline.ReplaceAllString(formatted, "\n\n")

	return strings.TrimSpace(formatted)
}

// GetCompactUserSummaryMessage 构建压缩后的摘要消息
// 参考: src/services/compact/prompt.ts:337-374
func GetCompactUserSummaryMessage(summary string, suppressFollowUp bool, transcriptPath string, recentMessagesPreserved bool) string {
	formattedSummary := FormatCompactSummary(summary)

	baseSummary := "This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.\n\n" + formattedSummary

	if transcriptPath != "" {
		baseSummary += "\n\nIf you need specific details from before compaction (like exact code snippets, error messages, or content you generated), read the full transcript at: " + transcriptPath
	}

	if recentMessagesPreserved {
		baseSummary += "\n\nRecent messages are preserved verbatim."
	}

	if suppressFollowUp {
		continuation := baseSummary + `
Continue the conversation from where it left off without asking the user any further questions. Resume directly — do not acknowledge the summary, do not recap what was happening, do not preface with "I'll continue" or similar. Pick up the last task as if the break never happened.`
		return continuation
	}

	return baseSummary
}