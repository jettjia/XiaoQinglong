package compactors

// ShouldCompactMicro 判断是否应该进行微压缩
func ShouldCompactMicro(messages []Message, tokenizer Tokenizer, threshold int) bool {
	for _, msg := range messages {
		if msg.Type != "user" {
			continue
		}
		for _, block := range msg.Content {
			if block.Type == "tool_result" {
				if content, ok := block.ToolResult.Content.(string); ok {
					if tokenizer.Estimate(content) > threshold {
						return true
					}
				}
			}
		}
	}
	return false
}