package main

import (
	"encoding/json"
	"io"

	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// a2uiWriter implements io.Writer to adapter a2ui.StreamToWriter to eventsChan
type a2uiWriter struct {
	eventsChan chan<- StreamEvent
}

func (w *a2uiWriter) Write(p []byte) (n int, err error) {
	w.eventsChan <- StreamEvent{Type: "a2ui", Data: map[string]any{"json": string(p)}}
	return len(p), nil
}

// ensure a2uiWriter implements io.Writer
var _ io.Writer = &a2uiWriter{}

// formatResponse 根据 response_schema 配置格式化响应
func (d *Dispatcher) formatResponse(content string) (string, []json.RawMessage) {
	if d.request.Options == nil || d.request.Options.ResponseSchema == nil {
		return content, nil
	}

	rs := d.request.Options.ResponseSchema
	logger.Infof("[Dispatcher] formatResponse: type=%s", rs.Type)

	switch rs.Type {
	case "a2ui":
		// 使用 schema 构建 A2UI 格式
		msgs := d.buildA2UIMessagesFromSchema(content, rs.Schema)
		logger.Infof("[Dispatcher] formatResponse: built %d a2ui messages", len(msgs))
		return "", msgs

	case "markdown", "text":
		// 直接返回 markdown 或文本内容
		return content, nil

	case "json":
		// 尝试解析 content 为 JSON 并美化输出
		var data any
		if err := json.Unmarshal([]byte(content), &data); err == nil {
			if prettyJSON, err := json.MarshalIndent(data, "", "  "); err == nil {
				return string(prettyJSON), nil
			}
		}
		return content, nil

	case "image", "audio", "video":
		// 多媒体格式 - 从 content 中解析 URL 或 base64
		// content 应该是 JSON 格式: {"url": "..."} 或 {"base64": "..."}
		var data map[string]any
		if err := json.Unmarshal([]byte(content), &data); err == nil {
			// 返回原始 JSON 作为 content
			if jsonStr, err := json.Marshal(data); err == nil {
				return string(jsonStr), nil
			}
		}
		return content, nil

	case "multipart":
		// 多格式混合 - content 应该是 JSON 数组格式
		return content, nil

	default:
		// 未知格式，返回原始内容
		logger.Infof("[Dispatcher] formatResponse: unknown type %s, returning raw content", rs.Type)
		return content, nil
	}
}

// buildA2UIMessagesFromSchema 根据 response_schema 构建 A2UI 格式消息
func (d *Dispatcher) buildA2UIMessagesFromSchema(content string, schema map[string]any) []json.RawMessage {
	msgs := []json.RawMessage{}

	// 解析 schema 获取 properties
	properties, _ := schema["properties"].(map[string]any)

	// 创建默认 surface
	surfaceID := "default_surface"

	createSurface, _ := json.Marshal(map[string]any{
		"createSurface": map[string]any{
			"surfaceId": surfaceID,
			"catalogId": "standard",
		},
	})
	msgs = append(msgs, createSurface)

	// 构建组件列表
	var components []map[string]any

	// 处理 content 字段 - 如果 schema 中定义了 content 字段，就使用它
	if properties != nil {
		if _, ok := properties["content"]; ok {
			// content 字段存在于 schema 中，添加文本组件
			components = append(components, map[string]any{
				"id":        "content",
				"component": "Text",
				"text":      map[string]any{"text": content},
			})
		}

		// 处理 action 字段
		if actionProp, ok := properties["action"].(map[string]any); ok {
			if actionType, _ := actionProp["type"].(string); actionType != "" {
				components = append(components, map[string]any{
					"id":         "action",
					"component":  "Action",
					"actionType": actionType,
				})
			}
		}

		// 处理 card 字段
		if cardProp, ok := properties["card"].(map[string]any); ok {
			if cardSchema, ok := cardProp["properties"].(map[string]any); ok {
				card := map[string]any{
					"id":        "card",
					"component": "Card",
				}
				if title, _ := cardSchema["title"].(string); title != "" {
					card["title"] = title
				}
				components = append(components, card)
			}
		}
	}

	// 如果没有从 schema 解析到组件，使用默认文本组件
	if len(components) == 0 {
		components = []map[string]any{
			{
				"id":        "text_content",
				"component": "Text",
				"text":      map[string]any{"text": content},
			},
		}
	}

	updateComponents, _ := json.Marshal(map[string]any{
		"updateComponents": map[string]any{
			"surfaceId":  surfaceID,
			"components": components,
		},
	})
	msgs = append(msgs, updateComponents)

	return msgs
}
