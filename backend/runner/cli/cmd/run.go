package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jettjia/XiaoQinglong/runner/cli/client"
	"github.com/jettjia/XiaoQinglong/runner/cli/config"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

// Run 单次执行
func Run(prompt string) error {
	if prompt == "" {
		// 从 stdin 读取
		data, err := os.Stdin.Read(make([]byte, 1024))
		if err != nil || data == 0 {
			return fmt.Errorf("no input provided")
		}
		prompt = strings.TrimSpace(string(data))
	}

	// 加载配置
	req, err := config.LoadConfig()
	if err != nil {
		return err
	}
	req.Prompt = prompt
	req.Options = &types.RunOptions{
		Stream: true,
	}

	// 创建 HTTP 客户端
	endpoint := config.GetEndpoint()
	runner := client.NewHTTPRunner(endpoint)

	// 运行
	ctx := context.Background()
	events, err := runner.RunStream(ctx, req)
	if err != nil {
		return err
	}

	// 处理流式事件
	for event := range events {
		switch event.Type {
		case "delta":
			if text, ok := event.Data["text"].(string); ok {
				fmt.Print(text)
			}
		case "tool_call":
			tool := event.Data["name"].(string)
			fmt.Printf("\n[Calling tool: %s]\n", tool)
		case "tool":
			tool := event.Data["name"].(string)
			fmt.Printf("[Tool %s returned]\n", tool)
		case "error":
			errMsg := event.Data["error"].(string)
			return fmt.Errorf(errMsg)
		case "done":
			fmt.Println()
		}
	}

	return nil
}
