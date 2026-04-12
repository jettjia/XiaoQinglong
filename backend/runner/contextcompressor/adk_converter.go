package contextcompressor

import (
	"github.com/cloudwego/eino/schema"
)

// ConvertToCCMessages 将 adk.Message 转换为 contextcompressor.Message
func ConvertToCCMessages(messages []*schema.Message) []Message {
	if messages == nil {
		return nil
	}

	result := make([]Message, 0, len(messages))
	for _, m := range messages {
		ccMsg := Message{
			Role: string(m.Role),
		}
		switch m.Role {
		case schema.User:
			ccMsg.Type = MessageTypeUser
			ccMsg.Content = []ContentBlock{{Type: "text", Text: m.Content}}
		case schema.Assistant:
			ccMsg.Type = MessageTypeAssistant
			ccMsg.Content = []ContentBlock{{Type: "text", Text: m.Content}}
		case schema.System:
			ccMsg.Type = MessageTypeSystem
			ccMsg.Content = []ContentBlock{{Type: "text", Text: m.Content}}
		}
		result = append(result, ccMsg)
	}
	return result
}

// ConvertFromCCMessages 将 contextcompressor.Message 转换回 adk.Message
func ConvertFromCCMessages(messages []Message) []*schema.Message {
	if messages == nil {
		return nil
	}

	result := make([]*schema.Message, 0, len(messages))
	for _, m := range messages {
		text := ""
		for _, block := range m.Content {
			if block.Type == "text" {
				text += block.Text
			}
		}
		switch m.Type {
		case MessageTypeUser:
			result = append(result, schema.UserMessage(text))
		case MessageTypeAssistant:
			result = append(result, schema.AssistantMessage(text, nil))
		case MessageTypeSystem:
			result = append(result, schema.SystemMessage(text))
		}
	}
	return result
}