// Package log 提供日志功能
package log

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	logger *log.Logger
	logFile *os.File
)

// Init 初始化日志（默认写入当前目录 passwordbox.log）
func Init() error {
	return InitWithPath(filepath.Join(".", "passwordbox.log"))
}

// InitWithPath 使用指定路径初始化日志
func InitWithPath(logPath string) error {
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// 如果无法创建文件，使用标准错误输出
		logger = log.New(os.Stderr, "[PasswordBox] ", log.LstdFlags)
		return fmt.Errorf("无法创建日志文件: %w", err)
	}

	logFile = file
	logger = log.New(file, "[PasswordBox] ", log.LstdFlags|log.Lshortfile)
	return nil
}

// Close 关闭日志文件
func Close() {
	if logFile != nil {
		logFile.Close()
	}
}

// Info 记录信息日志
func Info(format string, v ...interface{}) {
	if logger == nil {
		Init()
	}
	msg := fmt.Sprintf(format, v...)
	logger.Output(2, fmt.Sprintf("[INFO] %s", msg))
}

// Error 记录错误日志
func Error(format string, v ...interface{}) {
	if logger == nil {
		Init()
	}
	msg := fmt.Sprintf(format, v...)
	logger.Output(2, fmt.Sprintf("[ERROR] %s", msg))
}

// Debug 记录调试日志
func Debug(format string, v ...interface{}) {
	if logger == nil {
		Init()
	}
	msg := fmt.Sprintf(format, v...)
	logger.Output(2, fmt.Sprintf("[DEBUG] %s", msg))
}

// Warn 记录警告日志
func Warn(format string, v ...interface{}) {
	if logger == nil {
		Init()
	}
	msg := fmt.Sprintf(format, v...)
	logger.Output(2, fmt.Sprintf("[WARN] %s", msg))
}

// WithTime 带时间戳的日志
func WithTime(level string, format string, v ...interface{}) {
	if logger == nil {
		Init()
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("[%s] %s: %s\n", timestamp, level, msg)
}
