package logger

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/xqldir"
)

var (
	runnerLog     *logrus.Logger
	runnerLogOnce sync.Once
)

// IsRunnerLogEnabled checks if runner logging is enabled via environment variable
func IsRunnerLogEnabled() bool {
	return os.Getenv("RUNNER_PROXY_LOG") == "true"
}

// GetRunnerLogger returns a logger instance for runner proxy
// If RUNNER_PROXY_LOG=true, logs are written to ~/.xiaoqinglong/logs/run.log
// Otherwise, logs are discarded
func GetRunnerLogger() *logrus.Logger {
	runnerLogOnce.Do(func() {
		runnerLog = logrus.New()

		if IsRunnerLogEnabled() {
			// Ensure logs directory exists
			logsDir := xqldir.GetLogsDir()
			if err := os.MkdirAll(logsDir, 0755); err != nil {
				runnerLog.SetOutput(os.Stdout)
				runnerLog.WithError(err).Error("Failed to create logs directory, falling back to stdout")
				return
			}

			// Open run.log in unified logs directory
			logFile := filepath.Join(logsDir, "run.log")
			file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				runnerLog.SetOutput(os.Stdout)
				runnerLog.WithError(err).Error("Failed to open run.log, falling back to stdout")
				return
			}
			runnerLog.SetOutput(file)
			runnerLog.SetFormatter(&logrus.TextFormatter{
				FullTimestamp:   true,
				TimestampFormat: "2006/01/02 15:04:05",
			})
		} else {
			// Discard all logs
			runnerLog.SetOutput(os.NewFile(0, "/dev/null"))
		}
	})

	return runnerLog
}