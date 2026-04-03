package logger

import (
	"fmt"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	runnerLog     *logrus.Logger
	runnerLogOnce sync.Once
)

// IsRunnerLogEnabled checks if runner logging is enabled via environment variable
func IsRunnerLogEnabled() bool {
	return os.Getenv("RUNNER_LOG") == "true"
}

// GetRunnerLogger returns a logger instance for runner
// If RUNNER_LOG=true, logs are written to runner.log in current directory
// Otherwise, logs are discarded
func GetRunnerLogger() *logrus.Logger {
	runnerLogOnce.Do(func() {
		runnerLog = logrus.New()
		fmt.Printf("[DEBUG] GetRunnerLogger: RUNNER_LOG=%q, IsEnabled=%v\n", os.Getenv("RUNNER_LOG"), IsRunnerLogEnabled())

		if IsRunnerLogEnabled() {
			// Open runner.log file in current directory
			file, err := os.OpenFile("runner.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				runnerLog.SetOutput(os.Stdout)
				runnerLog.WithError(err).Error("Failed to open runner.log, falling back to stdout")
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

// Infof logs an info message
func Infof(format string, args ...any) {
	GetRunnerLogger().Infof(format, args...)
}

// Warnf logs a warning message
func Warnf(format string, args ...any) {
	GetRunnerLogger().Warnf(format, args...)
}

// Errorf logs an error message
func Errorf(format string, args ...any) {
	GetRunnerLogger().Errorf(format, args...)
}

// Debugf logs a debug message
func Debugf(format string, args ...any) {
	GetRunnerLogger().Debugf(format, args...)
}
