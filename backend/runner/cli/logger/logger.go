package logger

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Logger 日志记录器
type Logger struct {
	mu      sync.Mutex
	file    *os.File
	isDebug bool
}

var defaultLogger *Logger

// Init 初始化日志
func Init(logFilePath string, isDebug bool) error {
	defaultLogger = &Logger{
		isDebug: isDebug,
	}

	if logFilePath != "" {
		f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("open log file failed: %w", err)
		}
		defaultLogger.file = f
	}

	return nil
}

// Close 关闭日志
func Close() {
	if defaultLogger != nil && defaultLogger.file != nil {
		defaultLogger.file.Close()
	}
}

func now() string {
	return time.Now().Format("2006/01/02 15:04:05")
}

func formatLog(level, msg string) string {
	return fmt.Sprintf("[%s] %s: %s", now(), level, msg)
}

// Debug 调试日志
func Debug(format string, args ...any) {
	if defaultLogger == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if defaultLogger.isDebug {
		log.Print(formatLog("DEBUG", msg))
	}
	if defaultLogger.file != nil {
		defaultLogger.mu.Lock()
		defaultLogger.file.WriteString(formatLog("DEBUG", msg) + "\n")
		defaultLogger.mu.Unlock()
	}
}

// Info 信息日志
func Info(format string, args ...any) {
	if defaultLogger == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if defaultLogger.isDebug {
		log.Print(formatLog("INFO", msg))
	}
	if defaultLogger.file != nil {
		defaultLogger.mu.Lock()
		defaultLogger.file.WriteString(formatLog("INFO", msg) + "\n")
		defaultLogger.mu.Unlock()
	}
}

// Error 错误日志
func Error(format string, args ...any) {
	if defaultLogger == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if defaultLogger.isDebug {
		log.Print(formatLog("ERROR", msg))
	}
	if defaultLogger.file != nil {
		defaultLogger.mu.Lock()
		defaultLogger.file.WriteString(formatLog("ERROR", msg) + "\n")
		defaultLogger.mu.Unlock()
	}
}
