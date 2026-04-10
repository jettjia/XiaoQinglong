package compactors

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