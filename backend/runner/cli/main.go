package main

import (
	"fmt"
	"os"

	"github.com/jettjia/XiaoQinglong/runner/cli/cmd"
	"github.com/jettjia/XiaoQinglong/runner/cli/config"
	"github.com/jettjia/XiaoQinglong/runner/cli/logger"
)

func main() {
	// 初始化日志
	logFilePath := ""
	if config.IsLogFileMode() {
		logFilePath = config.GetLogFilePath()
	}
	if err := logger.Init(logFilePath, config.IsDebugMode()); err != nil {
		fmt.Fprintf(os.Stderr, "Init logger failed: %v\n", err)
	}
	defer logger.Close()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error

	switch os.Args[1] {
	case "chat":
		err = cmd.Chat()
	case "config":
		if len(os.Args) > 2 && os.Args[2] == "show" {
			err = cmd.ConfigShow()
		} else {
			printUsage()
			os.Exit(1)
		}
	case "run":
		prompt := ""
		if len(os.Args) > 2 {
			prompt = os.Args[2]
		}
		err = cmd.Run(prompt)
	case "stop":
		if len(os.Args) < 3 {
			fmt.Println("Error: checkpoint_id required")
			fmt.Println("Usage: runcli stop <checkpoint_id>")
			os.Exit(1)
		}
		err = cmd.Stop(os.Args[2])
	case "-h", "--help", "help":
		printUsage()
	default:
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		logger.Error("Command error: %v", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Runner CLI - Command line interface for Runner

Usage:
  runcli chat              Start interactive chat mode
  runcli config show       Show current configuration
  runcli run [prompt]      Run a single prompt (reads from stdin if no prompt provided)
  runcli stop <checkpoint_id>  Stop a running task

Environment Variables:
  RUNNER_MODE=http              Set to "http" for HTTP mode (default)
  RUNNER_HTTP_ENDPOINT=...      HTTP endpoint (default: http://localhost:18080)
  RUNNER_MODEL_DEFAULT_NAME=... Default model name
  RUNNER_MODEL_DEFAULT_APIKEY=... API key
  RUNNER_MODEL_DEFAULT_APIBASE=... API base URL
  RUNNER_MODEL_DEFAULT_TEMPERATURE=... Temperature
  RUNNER_MODEL_DEFAULT_MAXTOKENS=... Max tokens
  RUNNER_MODEL_SKILL_NAME=...   Skill model name
  RUNNER_DEFAULT_MODEL=default  Default model role

  RUNNER_DEBUG=false            Enable debug mode (print logs to console)
  RUNNER_LOG_FILE=true          Enable log file (default: true)
  RUNNER_LOG_FILE_PATH=cli.log Log file path

Examples:
  export RUNNER_MODEL_DEFAULT_NAME=gpt-4o
  export RUNNER_MODEL_DEFAULT_APIKEY=${OPENAI_API_KEY}
  export RUNNER_MODEL_DEFAULT_APIBASE=https://api.openai.com/v1

  runcli chat                # Start interactive chat
  runcli config show         # Show config
  echo "Hello!" | runcli run # Run single prompt
`)
}
