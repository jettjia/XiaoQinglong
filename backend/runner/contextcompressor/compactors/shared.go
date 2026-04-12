package compactors

import (
	"fmt"
	"strings"
)

// StripImagesFromMessages 剥离图片和文档，替换为标记
// 参考: src/services/compact/compact.ts:145-200
func StripImagesFromMessages(messages []Message) []Message {
	result := make([]Message, len(messages))
	for i, msg := range messages {
		if msg.Type != "user" {
			result[i] = msg
			continue
		}

		newContent := make([]ContentBlock, 0)
		for _, block := range msg.Content {
			switch block.Type {
			case "image":
				newContent = append(newContent, ContentBlock{Type: "text", Text: "[image]"})
			case "document":
				newContent = append(newContent, ContentBlock{Type: "text", Text: "[document]"})
			default:
				newContent = append(newContent, block)
			}
		}
		result[i] = msg
		result[i].Content = newContent
	}
	return result
}

// PruneToolResults 裁剪旧的工具结果，替换为占位符
// 压缩的第一步：删除旧工具调用的详细结果
// 参考: agent/context_compressor.py
func PruneToolResults(messages []Message, maxResultLen int) []Message {
	if maxResultLen <= 0 {
		maxResultLen = 500 // 默认最大工具结果长度
	}

	result := make([]Message, len(messages))
	for i, msg := range messages {
		result[i] = msg
		newContent := make([]ContentBlock, 0, len(msg.Content))

		for _, block := range msg.Content {
			if block.Type == "text" {
				newContent = append(newContent, block)
			} else if block.ToolResult != nil {
				// 裁剪工具结果
				content := ""
				switch v := block.ToolResult.Content.(type) {
				case string:
					content = v
				default:
					content = "[complex result]"
				}

				if len(content) > maxResultLen {
					content = content[:maxResultLen] + "... [truncated]"
				}
				newContent = append(newContent, ContentBlock{
					Type: "text",
					Text: "[tool result: " + content + "]",
				})
			} else {
				newContent = append(newContent, block)
			}
		}
		result[i].Content = newContent
	}
	return result
}

// ProtectHeadMessages 保护头部消息（系统消息 + 前几个对话轮次）
// 这些消息不被压缩，保持原样
func ProtectHeadMessages(messages []Message, protectCount int) ([]Message, []Message) {
	if protectCount <= 0 {
		protectCount = 3 // 默认保护前3条消息
	}
	if len(messages) <= protectCount {
		return messages, nil
	}
	return messages[:protectCount], messages[protectCount:]
}

// ProtectTailMessages 保护尾部消息（最近的消息，保留足够的 token budget）
// 只保留足够的最近消息来填满 token budget
func ProtectTailMessages(messages []Message, tokenizer Tokenizer, maxTailTokens int) ([]Message, []Message) {
	if maxTailTokens <= 0 {
		maxTailTokens = 20000 // 默认保留最近 20000 tokens
	}
	if len(messages) == 0 {
		return nil, nil
	}

	// 从后向前扫描，计算 token 数
	var toKeep []Message
	tokenCount := 0
	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := tokenizer.EstimateMessages([]Message{messages[i]})
		if tokenCount+msgTokens > maxTailTokens {
			break
		}
		toKeep = append([]Message{messages[i]}, toKeep...)
		tokenCount += msgTokens
	}

	// 消息分为保护部分和非保护部分
	if len(toKeep) >= len(messages) {
		return nil, messages
	}
	return messages[:len(messages)-len(toKeep)], toKeep
}

// GetLastText 获取消息的最后一条文本
func (m *Message) GetLastText() string {
	if m == nil {
		return ""
	}
	for i := len(m.Content) - 1; i >= 0; i-- {
		if m.Content[i].Type == "text" && m.Content[i].Text != "" {
			return m.Content[i].Text
		}
	}
	return ""
}

// IsToolMessage 检查消息是否为工具调用或工具结果
func IsToolMessage(msg *Message) bool {
	if msg == nil {
		return false
	}
	for _, block := range msg.Content {
		if block.ToolUse != nil || block.ToolResult != nil {
			return true
		}
	}
	return false
}

// CountMessagePairs 计算对话轮次（用户+助手对）
func CountMessagePairs(messages []Message) int {
	pairs := 0
	for i := 0; i < len(messages)-1; i++ {
		if messages[i].Role == "user" && messages[i+1].Role == "assistant" {
			pairs++
		}
	}
	return pairs
}

// TruncateText 截断文本到指定长度
func TruncateText(text string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 1000
	}
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "... [truncated]"
}

// ExtractToolNames 提取消息中所有的工具名称
func ExtractToolNames(messages []Message) []string {
	tools := make(map[string]bool)
	for _, msg := range messages {
		for _, block := range msg.Content {
			if block.ToolUse != nil && block.ToolUse.Name != "" {
				tools[block.ToolUse.Name] = true
			}
		}
	}
	result := make([]string, 0, len(tools))
	for name := range tools {
		result = append(result, name)
	}
	return result
}

// FormatMessagesForSummary 将消息格式化为摘要输入
// 使用结构化格式：Goal, Progress, Decisions, Files, Next Steps
func FormatMessagesForSummary(messages []Message) string {
	var sb strings.Builder

	// 按对话轮次分组
	var userMsgs []Message
	var assistantMsgs []Message

	for _, msg := range messages {
		if msg.Role == "user" {
			userMsgs = append(userMsgs, msg)
		} else if msg.Role == "assistant" {
			assistantMsgs = append(assistantMsgs, msg)
		}
	}

	sb.WriteString("## Conversation Summary\n\n")

	// 提取工具调用
	tools := ExtractToolNames(messages)
	if len(tools) > 0 {
		sb.WriteString("### Tools Used:\n")
		for _, tool := range tools {
			sb.WriteString("- " + tool + "\n")
		}
		sb.WriteString("\n")
	}

	// 总结对话
	sb.WriteString("### Exchange Summary:\n")
	pairs := min(len(userMsgs), len(assistantMsgs))
	for i := 0; i < pairs; i++ {
		userText := TruncateText(userMsgs[i].GetLastText(), 200)
		assistantText := TruncateText(assistantMsgs[i].GetLastText(), 200)
		sb.WriteString(fmt.Sprintf("%d. User: %s\n   Assistant: %s\n\n", i+1, userText, assistantText))
	}

	return sb.String()
}
