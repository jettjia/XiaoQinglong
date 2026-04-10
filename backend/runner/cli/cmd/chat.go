package cmd

import (
	"context"

	"github.com/jettjia/XiaoQinglong/runner/cli/client"
	"github.com/jettjia/XiaoQinglong/runner/cli/config"
	"github.com/jettjia/XiaoQinglong/runner/cli/repl"
)

// Chat 运行交互式对话
func Chat() error {
	// 加载并验证配置
	if _, err := config.LoadConfig(); err != nil {
		return err
	}

	// 创建 HTTP 客户端
	endpoint := config.GetEndpoint()
	runner := client.NewHTTPRunner(endpoint)

	// 创建 REPL
	repl := repl.NewREPL(runner)

	// 运行
	ctx := context.Background()
	return repl.Run(ctx)
}
