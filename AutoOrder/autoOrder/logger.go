package autoOrder

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// Logger 是一个自定义的日志记录器结构体
type Logger struct {
	logger *log.Logger
}

// NewLogger 创建一个返回到标准输出的Logger实例
func NewLogger(out io.Writer, prefix string, flag int) *Logger {
	return &Logger{
		logger: log.New(out, prefix, flag),
	}
}

// NewFileLogger 创建一个将日志输出到指定文件的Logger实例
func NewFileLogger(logPath string) (*Logger, error) {
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return &Logger{
		logger: log.New(file, "", 0),
	}, nil
}

// 错误信息
func (l *Logger) WriteError(v ...interface{}) {
	now := time.Now().Format("2006-01-02 15:04:05")
	l.logger.Println("[ERROR]" + now + " " + fmt.Sprint(v...))
}

// 实时信息
func (l *Logger) WriteInfo(v ...interface{}) {
	now := time.Now().Format("2006-01-02 15:04:05")
	l.logger.Println("[INFO]" + now + " " + fmt.Sprint(v...))
}
