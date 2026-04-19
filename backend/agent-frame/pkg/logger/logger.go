package logger

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/xqldir"
)

var (
	runnerLog     *logrus.Logger
	runnerLogOnce sync.Once
)

// errorLogHook is a logrus hook that writes warning+ entries to a separate error log file
type errorLogHook struct {
	writer *lumberjack.Logger
	levels []logrus.Level
}

func (h *errorLogHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		return err
	}
	_, err = h.writer.Write([]byte(line))
	return err
}

func (h *errorLogHook) Levels() []logrus.Level {
	return h.levels
}

// IsRunnerLogEnabled checks if runner logging is enabled via environment variable
func IsRunnerLogEnabled() bool {
	return os.Getenv("RUNNER_PROXY_LOG") == "true"
}

// GetRunnerLogger returns a logger instance for runner proxy
// If RUNNER_PROXY_LOG=true, logs are written to ~/.xiaoqinglong/logs/run.log with rotation
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

			// Main log file - INFO+ level with rotation
			logFile := filepath.Join(logsDir, "run.log")
			runnerLog.SetOutput(&lumberjack.Logger{
				Filename:   logFile,
				MaxSize:    10, // 10 MB per file
				MaxAge:     7,  // 7 days retention
				MaxBackups: 5,  // keep 5 backup files
				Compress:   true,
				LocalTime:  true,
			})
			runnerLog.SetFormatter(&logrus.TextFormatter{
				FullTimestamp:   true,
				TimestampFormat: "2006/01/02 15:04:05",
			})
			runnerLog.SetLevel(logrus.InfoLevel)

			// Error log file - WARNING+ level with rotation
			errorLogFile := filepath.Join(logsDir, "run.error.log")
			runnerLog.AddHook(&errorLogHook{
				writer: &lumberjack.Logger{
					Filename:   errorLogFile,
					MaxSize:    5,  // 5 MB per file
					MaxAge:     7,  // 7 days retention
					MaxBackups: 3,  // keep 3 backup files
					Compress:   true,
					LocalTime:  true,
				},
				levels: []logrus.Level{logrus.WarnLevel, logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel},
			})
		} else {
			// Discard all logs
			runnerLog.SetOutput(os.NewFile(0, "/dev/null"))
		}
	})

	return runnerLog
}