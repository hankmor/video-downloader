package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	logFile      *os.File
	logPath      string
	currentLevel Level = LevelInfo // 默认 INFO
)

// SetLevel 设置日志级别
func SetLevel(l Level) {
	currentLevel = l
}

// Init 初始化日志系统
// customLogPath: 可选参数，指定日志文件路径。如果不传递或为空字符串，则使用默认路径
func Init(customLogPath ...string) error {
	var err error

	// 如果提供了自定义日志路径，使用它；否则使用默认路径
	if len(customLogPath) > 0 && customLogPath[0] != "" {
		logPath = customLogPath[0]

		// 确保日志文件所在目录存在
		logDir := filepath.Dir(logPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("创建日志目录失败: %w", err)
		}
	} else {
		// 使用默认路径
		// 确定日志目录
		logDir, err := getLogDir()
		if err != nil {
			return fmt.Errorf("无法获取日志目录: %w", err)
		}

		// 确保目录存在
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("创建日志目录失败: %w", err)
		}

		// 日志文件路径 (app.log)
		// 如果需要按日期轮转，可以在这里加上日期后缀
		logPath = filepath.Join(logDir, "app.log")
	}

	// 打开日志文件 (追加模式)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开日志文件: %w", err)
	}
	logFile = f

	// 设置多重输出 (文件 + 控制台)
	multiWriter := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multiWriter)

	// 设置日志格式 (日期 时间 文件名:行号)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	return nil
}

// outputLog 内部日志输出函数
func outputLog(level Level, prefix string, format string, v ...any) {
	if level < currentLevel {
		return
	}
	msg := fmt.Sprintf(format, v...)
	log.Output(3, fmt.Sprintf("%s %s", prefix, msg))
}

// Debug 调试日志
func Debug(v ...any) {
	outputLog(LevelDebug, "[DEBUG]", "%s", fmt.Sprint(v...))
}

// Debugf 调试日志 (格式化)
func Debugf(format string, v ...any) {
	outputLog(LevelDebug, "[DEBUG]", format, v...)
}

// Info 信息日志
func Info(v ...any) {
	outputLog(LevelInfo, "[INFO]", "%s", fmt.Sprint(v...))
}

// Infof 信息日志 (格式化)
func Infof(format string, v ...any) {
	outputLog(LevelInfo, "[INFO]", format, v...)
}

// Warn 警告日志
func Warn(v ...any) {
	outputLog(LevelWarn, "[WARN]", "%s", fmt.Sprint(v...))
}

// Warnf 警告日志 (格式化)
func Warnf(format string, v ...any) {
	outputLog(LevelWarn, "[WARN]", format, v...)
}

// Error 错误日志
func Error(v ...any) {
	outputLog(LevelError, "[ERROR]", "%s", fmt.Sprint(v...))
}

// Errorf 错误日志 (格式化)
func Errorf(format string, v ...any) {
	outputLog(LevelError, "[ERROR]", format, v...)
}

// Printf 兼容标准库 log.Printf，默认使用 INFO 级别
func Printf(format string, v ...any) {
	Infof(format, v...)
}

// Println 兼容标准库 log.Println，默认使用 INFO 级别
func Println(v ...any) {
	Info(v...)
}

// GetLogPath 获取当前日志文件路径
func GetLogPath() string {
	return logPath
}

// Close 关闭日志文件
func Close() {
	if logFile != nil {
		logFile.Close()
	}
}

// getLogDir 获取日志存储目录
// 优先使用 os.UserConfigDir/VDD/logs, 失败则回退到 ~/.vdd/logs
func getLogDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err == nil {
		return filepath.Join(configDir, "VDD", "logs"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".vdd", "logs"), nil
}
