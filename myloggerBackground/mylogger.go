package myLogger

// 后台异步写日志，避免阻塞正式运行的程序
import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// 定义日志级别类型
type logLevel uint16

// 定义日志级别常量
const (
	UNKNOWN logLevel = iota
	DEBUG
	TRACE
	INFO
	WARNING
	ERROR
	FATAL
)

// Logger 定义个日志结构体（对象）接口，必须要实现的方法
type Logger interface {
	Debug(formatMsg string, a ...any)
	Trace(formatMsg string, a ...any)
	Info(formatMsg string, a ...any)
	Warning(formatMsg string, a ...any)
	Error(formatMsg string, a ...any)
	Fatal(formatMsg string, a ...any)
}

// ConsoleLogger 定义个日志结构体（对象）给
type ConsoleLogger struct {
	Level logLevel
}

// logMsg 日志信息结构体，异步写日志用
type logMsg struct {
	logLevel logLevel
	nowStr   string
	funcName string
	fileName string
	lineNum  int
	msg      string
}

// FileLogger 文件日志结构体
type FileLogger struct {
	level         logLevel
	logPath       string
	logName       string
	maxFileSize   int64
	logFileObj    *os.File
	errLogFileObj *os.File
	logChan       chan *logMsg // 缓冲区通道
}

// parseLogLevelStr 解析日志级别字符串为日志级别类型
func parseLogLevelStr(s string) (logLevel, error) {
	s = strings.ToLower(s)
	switch s {
	case "debug":
		return DEBUG, nil
	case "trace":
		return TRACE, nil
	case "info":
		return INFO, nil
	case "warning":
		return WARNING, nil
	case "error":
		return ERROR, nil
	case "fatal":
		return FATAL, nil
	default:
		err := errors.New("日志级别错误")
		return UNKNOWN, err
	}
}

// parseLogLevel 将日志级别对象转换为字符串
func parseLogLevel(l logLevel) (string, error) {
	switch l {
	case DEBUG:
		return "DEBUG", nil
	case TRACE:
		return "TRACE", nil
	case INFO:
		return "INFO", nil
	case WARNING:
		return "WARNING", nil
	case ERROR:
		return "ERROR", nil
	case FATAL:
		return "FATAL", nil
	default:
		err := errors.New("日志级别错误")
		return "UNKNOWN", err
	}
}

// NewConsoleLogger 创建一个Logger对象的方法
func NewConsoleLogger(levelStr string) Logger {
	level, err := parseLogLevelStr(levelStr)
	if err != nil {
		panic(err)
	}
	return ConsoleLogger{Level: level}
}

// NewFileLogger 创建一个FileLogger对象的方法
func NewFileLogger(levelStr, path, logFileName string, maxSize int64) (fileLogger *FileLogger) {
	level, err := parseLogLevelStr(levelStr)
	if err != nil {
		panic(err)
	}
	fileLogger = &FileLogger{
		level:       level,
		logPath:     path,
		logName:     logFileName,
		maxFileSize: maxSize,
	}
	err = fileLogger.initFileObj()
	if err != nil {
		panic(err)
	}
	return
}

// getInfo 获取调用日志含数的文件名和函数，记录日志的语句行号
func getInfo(skip int) (funcName, fileName string, lineNum int) {
	//pc uintptr, file string, line int, ok
	pc, file, line, ok := runtime.Caller(skip)
	//方法名
	funcName = runtime.FuncForPC(pc).Name()
	funcName = strings.Split(funcName, ".")[1]
	//调用写日志的代码文件名
	fileName = file
	//fileName = path.Base(file)
	// 调用写日志的代码行号line
	if !ok {
		fmt.Println("getInfo runtime.Caller Failed")
		return
	}
	return funcName, fileName, line
}
