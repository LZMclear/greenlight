package jsonlog

import (
	"encoding/json"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

// Level 表示日志条目的严重级别
type Level int8

// 初始化表示特定严重性级别的常量
const (
	LevelInfo Level = iota
	LevelError
	LevelFatal
	LevelOff
)

// 返回一个对人类友好的严重级别字符串
func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return ""
	}
}

// Logger 定义一个自定义日志类型，保存了日志条目要写入的地点，写入的最小日志级别
type Logger struct {
	out      io.Writer
	minLevel Level
	mu       sync.Mutex
}

// New 返回一个Logger实例
func New(out io.Writer, minLevel Level) *Logger {
	return &Logger{
		out:      out,
		minLevel: minLevel,
	}
}

// 用来编写不同级别的日志条目

func (l *Logger) print(level Level, message string, properties map[string]string) (int, error) {
	if level < l.minLevel {
		return 0, nil
	}
	//声明一个匿名结构体用来保存日志信息
	aux := struct {
		Level      string            `json:"level"`
		Time       string            `json:"time"`
		Message    string            `json:"message"`
		Properties map[string]string `json:"properties"`
		Trace      string            `json:"trace"`
	}{
		Level:      level.String(),
		Time:       time.Now().UTC().Format(time.RFC3339),
		Message:    message,
		Properties: properties,
	}
	//为Error和Fatal级别的日志添加堆栈跟踪
	if level >= LevelError {
		aux.Trace = string(debug.Stack())
	}
	//用来保存实际的日志文本
	var line []byte
	//将日志信息编码为json格式
	line, err := json.Marshal(aux)
	if err != nil {
		line = []byte(LevelError.String() + ":unable to marshal log message:" + err.Error())
	}
	//锁定互斥锁，避免两次写入操作
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.out.Write(append(line, '\n'))
}

// 实现Write方法，使其成为io.Write接口。使其可以与http.Server错误日志集成
func (l *Logger) Write(message []byte) (n int, err error) {
	return l.print(LevelError, string(message), nil)
}
func (l *Logger) PrintInfo(message string, properties map[string]string) {
	l.print(LevelInfo, message, properties)
}
func (l *Logger) PrintError(err error, properties map[string]string) {
	l.print(LevelError, err.Error(), properties)
}
func (l *Logger) PrintFatal(err error, properties map[string]string) {
	l.print(LevelFatal, err.Error(), properties)
	os.Exit(1) // For entries at the FATAL level, we also terminate the application.
}
