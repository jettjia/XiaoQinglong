package repl

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jettjia/XiaoQinglong/runner/cli/client"
	"github.com/jettjia/XiaoQinglong/runner/cli/config"
	"github.com/jettjia/XiaoQinglong/runner/cli/logger"
	"github.com/jettjia/XiaoQinglong/runner/types"
)

// REPL 交互式对话
type REPL struct {
	runner     *client.HTTPRunner
	messages   []types.Message
	checkpoint string
	stdin      *bufio.Reader
	stdout     *bufio.Writer
}

// NewREPL 创建 REPL
func NewREPL(runner *client.HTTPRunner) *REPL {
	return &REPL{
		runner:   runner,
		messages: make([]types.Message, 0),
		stdin:    bufio.NewReader(os.Stdin),
		stdout:   bufio.NewWriter(os.Stdout),
	}
}

// Run 启动 REPL
func (r *REPL) Run(ctx context.Context) error {
	r.printBanner()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 显示提示符
		r.printPrompt()

		// 读取用户输入
		input, err := r.stdin.ReadString('\n')
		if err != nil {
			if err.Error() == "EOF" {
				fmt.Println("\nGoodbye!")
				return nil
			}
			return err
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// 处理特殊命令
		if r.handleCommand(input) {
			continue
		}

		// 添加用户消息
		r.messages = append(r.messages, types.Message{
			Role:    "user",
			Content: input,
		})

		// 发送请求
		if err := r.runStream(ctx, input); err != nil {
			logger.Error("Run error: %v", err)
			fmt.Fprintf(r.stdout, "Error: %v\n", err)
			r.stdout.Flush()
		}
	}
}

// runStream 流式运行
func (r *REPL) runStream(ctx context.Context, prompt string) error {
	// 加载配置以获取 Models
	req, err := config.LoadConfig()
	if err != nil {
		return err
	}

	// 设置当前输入
	req.Prompt = prompt
	req.Messages = r.messages
	req.Options = &types.RunOptions{
		Stream:       true,
		CheckPointID: r.checkpoint,
	}

	logger.Info("Sending request: prompt=%s, messages_count=%d", prompt, len(r.messages))

	events, err := r.runner.RunStream(ctx, req)
	if err != nil {
		return err
	}

	var assistantMsg strings.Builder

	// 处理流式事件
	for event := range events {
		switch event.Type {
		case "delta":
			if text, ok := event.Data["text"].(string); ok {
				fmt.Print(text)
				assistantMsg.WriteString(text)
			}
		case "tool_call":
			if tool, ok := event.Data["tool"].(string); ok {
				fmt.Fprintf(r.stdout, "\n[Calling tool: %s]\n", tool)
				logger.Debug("Tool call: %s", tool)
			}
		case "tool":
			tool := ""
			if t, ok := event.Data["tool"].(string); ok {
				tool = t
			}
			output := event.Data["output"]
			outputStr := ""
			if s, ok := output.(string); ok {
				outputStr = s
			} else {
				outputStr = fmt.Sprintf("%v", output)
			}
			if len(outputStr) > 200 {
				outputStr = outputStr[:200] + "..."
			}
			fmt.Fprintf(r.stdout, "[Tool %s returned: %s]\n", tool, outputStr)
			logger.Debug("Tool result: %s = %s", tool, outputStr)
		case "interrupted":
			if id, ok := event.Data["interrupt_id"].(string); ok {
				r.checkpoint = id
				fmt.Fprintf(r.stdout, "\n[Interrupted. Checkpoint: %s]\n", r.checkpoint)
				logger.Info("Interrupted, checkpoint=%s", r.checkpoint)
				r.stdout.Flush()
			}
			return nil
		case "error":
			errMsg := ""
			if e, ok := event.Data["error"].(string); ok {
				errMsg = e
			}
			logger.Error("Error event: %s", errMsg)
			fmt.Fprintf(r.stdout, "\n[Error: %s]\n", errMsg)
			return fmt.Errorf(errMsg)
		case "done":
			r.checkpoint = ""
			logger.Info("Done, response_length=%d", assistantMsg.Len())
			fmt.Println()
		}
		r.stdout.Flush()
	}

	// 保存助手消息
	if assistantMsg.Len() > 0 {
		r.messages = append(r.messages, types.Message{
			Role:    "assistant",
			Content: assistantMsg.String(),
		})
	}

	return nil
}

// handleCommand 处理特殊命令
func (r *REPL) handleCommand(input string) bool {
	switch strings.ToLower(input) {
	case "/quit", "/exit", "/q":
		fmt.Println("Goodbye!")
		os.Exit(0)
		return false // unreachable
	case "/clear":
		r.messages = nil
		fmt.Println("Conversation cleared.")
		logger.Info("Conversation cleared")
		return true
	case "/history":
		for i, m := range r.messages {
			role := m.Role
			content := m.Content
			if len(content) > 80 {
				content = content[:80] + "..."
			}
			fmt.Fprintf(r.stdout, "%d. [%s]: %s\n", i+1, role, content)
		}
		r.stdout.Flush()
		return true
	case "/help":
		fmt.Println(`Commands:
  /quit, /exit, /q  - Exit the chat
  /clear            - Clear conversation history
  /history          - Show conversation history
  /help             - Show this help message`)
		r.stdout.Flush()
		return true
	default:
		return false
	}
}

// printBanner 打印欢迎信息
func (r *REPL) printBanner() {
	fmt.Println("=== Runner CLI ===")
	fmt.Println("Type /help for commands, /quit to exit.")
	fmt.Println()
}

// printPrompt 打印提示符
func (r *REPL) printPrompt() {
	fmt.Print("> ")
	r.stdout.Flush()
}
