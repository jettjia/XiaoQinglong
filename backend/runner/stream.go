package main

import (
	"context"
)

// StreamEvent 流式事件
type StreamEvent struct {
	Type string
	Data map[string]any
}

// eventsChanKey 用于在 context 中传递 events channel
type eventsChanKeyType string

const eventsChanKey eventsChanKeyType = "events_chan"

// withEventsChan 将 events channel 存入 context
func withEventsChan(ctx context.Context, ch chan StreamEvent) context.Context {
	return context.WithValue(ctx, eventsChanKey, ch)
}

// getEventsChan 从 context 获取 events channel
func getEventsChan(ctx context.Context) chan StreamEvent {
	ch, _ := ctx.Value(eventsChanKey).(chan StreamEvent)
	return ch
}
