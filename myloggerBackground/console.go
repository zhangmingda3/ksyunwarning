package myLogger

import (
	"fmt"
	"time"
)

// 判断日志级别是否开启的方法
func (l ConsoleLogger) enable(logLevel logLevel) bool {
	return logLevel >= l.Level
}

// 写日志
func (l ConsoleLogger) write(logLevel logLevel, formatMsg string, a ...any) {
	if l.enable(logLevel) {
		nowStr := time.Now().Format("2006-01-02 15:04:05")
		funcName, fileName, lineNum := getInfo(3)
		logLevelStr, _ := parseLogLevel(logLevel)
		msg := fmt.Sprintf(formatMsg, a...) // 格式化字符串
		fmt.Printf("[%s] [%v] [%s:%s:line:%d] %s\n", nowStr, logLevelStr, funcName, fileName, lineNum, msg)
		//fmt.Printf("l ConsoleLogger的地址是：%v",&l)
	}
}

func (l ConsoleLogger) Debug(formatMsg string, a ...any) {
	l.write(DEBUG, formatMsg, a...)

}
func (l ConsoleLogger) Trace(formatMsg string, a ...any) {
	l.write(TRACE, formatMsg, a...)
}

func (l ConsoleLogger) Info(formatMsg string, a ...any) {
	l.write(INFO, formatMsg, a...)
}

func (l ConsoleLogger) Warning(formatMsg string, a ...any) {
	l.write(WARNING, formatMsg, a...)
}

func (l ConsoleLogger) Error(formatMsg string, a ...any) {
	l.write(ERROR, formatMsg, a...)
}

func (l ConsoleLogger) Fatal(formatMsg string, a ...any) {
	l.write(FATAL, formatMsg, a...)
}
