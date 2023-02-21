package myLogger

import (
	"fmt"
	"os"
	"path"
	"time"
)

// enable 判断日志级别是否开启的方法
func (l *FileLogger) enable(logLevel logLevel) bool {
	return logLevel >= l.level
}

// initFileObj 把日志文件打开,打不开就panic
func (l *FileLogger) initFileObj() error {
	fullFileName := path.Join(l.logPath, l.logName)
	fmt.Println(fullFileName)
	fileObj, err := os.OpenFile(fullFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("open logFile Failed- %v", err)
		return err
	}
	errFileObj, err := os.OpenFile(fullFileName+".err", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("open ErrorLogFile Failed- %v", err)
		return err
	}
	l.logFileObj = fileObj
	l.errLogFileObj = errFileObj
	// 初始化日志缓冲通道
	l.logChan = make(chan *logMsg, 50000)
	// 启动一个goroutine 从通道中取日志写入磁盘
	go l.asyncWriteLog()
	return nil
}

// Close 关闭文件 (关闭普通日志和错误日志文件、)
//func (l *FileLogger) Close() {
//	err := l.logFileObj.Close()
//	if err != nil {
//		panic(err)
//	}
//	err = l.errLogFileObj.Close()
//	if err != nil {
//		panic(err)
//	}
//}

// needSplit 判断是否需要切割
func (l *FileLogger) needSplit(f *os.File) bool {
	fileInfo, err := f.Stat()
	if err != nil {
		//fmt.Println("判断打开的日志",f)
		//fmt.Printf("获取日志文件信息错误\n")
		panic(err)
	}
	if fileInfo != nil {
		fileSize := fileInfo.Size()
		//fmt.Printf("%s文件大小:%v字节",fileInfo.Name(),fileSize)
		return fileSize > l.maxFileSize
	}
	panic("判断是否需要切割崩溃...")
}

// needSplitByHour 判断是否需要按时间切割
func (l *FileLogger) needSplitByHour(f *os.File) bool {
	fileInfo, err := f.Stat()
	if err != nil {
		//fmt.Println("判断打开的日志",f)
		//fmt.Printf("获取日志文件信息错误\n")
		panic(err)
	}
	if fileInfo != nil {
		fileHour := fileInfo.ModTime().Hour()
		nowHour := time.Now().Hour()
		//fmt.Printf("%s文件大小:%v字节",fileInfo.Name(),fileSize)
		return nowHour > fileHour
	}
	panic("判断是否需要切割崩溃...")
}

// splitLogFile 切割日志文件(按文件大小)
func (l *FileLogger) splitLogFile(f *os.File) *os.File {
	//fmt.Println("要切割的文件：",f)
	fileInfo, err := f.Stat()
	if err != nil {
		fmt.Printf("获取日志文件信息错误")
		panic(err)
	}
	if fileInfo != nil {
		fileName := fileInfo.Name()
		nowStr := time.Now().Format("2006-01-02-15h04m05s")
		backFileName := fileName + "_" + nowStr + ".log"
		backFileAbsPath := path.Join(l.logPath, backFileName)
		logFileAbsPath := path.Join(l.logPath, fileName)
		//fmt.Println(logFileAbsPath)
		err = f.Close()
		if err != nil {
			panic(err)
		}
		err = os.Rename(logFileAbsPath, backFileAbsPath)
		if err != nil {
			panic(err)
		}
		fileObj, err := os.OpenFile(logFileAbsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		//fmt.Printf("成功创建新文件\n")
		return fileObj // 文件创建成功
	}
	return nil
}

// splitLogFileByHour 按时间切割日志文件
func (l *FileLogger) splitLogFileByHour(f *os.File) *os.File {
	//fmt.Println("要切割的文件：",f)
	fileInfo, err := f.Stat()
	if err != nil {
		fmt.Printf("获取日志文件信息错误")
		panic(err)
	}
	if fileInfo != nil {
		fileName := fileInfo.Name()
		fileHourTime := fileInfo.ModTime().Format("2006-01-02-15")
		backFileName := fileName + "_" + fileHourTime
		backFileAbsPath := path.Join(l.logPath, backFileName)
		logFileAbsPath := path.Join(l.logPath, fileName)
		//fmt.Println(logFileAbsPath)
		err = f.Close()
		if err != nil {
			panic(err)
		}
		err = os.Rename(logFileAbsPath, backFileAbsPath)
		if err != nil {
			panic(err)
		}
		fileObj, err := os.OpenFile(logFileAbsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		//fmt.Printf("成功创建新文件\n")
		return fileObj // 文件创建成功
	}
	return nil
}

// asyncWriteLog 异步写日志
func (l *FileLogger) asyncWriteLog() {
	//写个死循环一直从队列里面拿日志
	for {
		////按日志文件大小切割
		if l.needSplit(l.logFileObj) {
			fmt.Printf("l 地址是%v\n", &l)
			newFileObj := l.splitLogFile(l.logFileObj)
			if newFileObj == nil {
				panic("新建日志打开日志文件返回为空")
			}
			//fmt.Println("已打开文件内存地址：",l.logFileObj)
			l.logFileObj = newFileObj
		}
		//按日志时间切割
		//if l.needSplitByHour(l.logFileObj){
		//	fmt.Printf("l 地址是%v\n",&l)
		//	newFileObj := l.splitLogFileByHour(l.logFileObj)
		//	if newFileObj == nil {
		//		panic("新建日志打开日志文件返回为空")
		//	}
		//	//fmt.Println("已打开文件内存地址：",l.logFileObj)
		//	l.logFileObj = newFileObj
		//	//fmt.Println("更新后文件内存地址：",l.logFileObj)
		//}
		select { // 如果取不到日志，就default ： 让出CPU一会
		case logMsg := <-l.logChan:
			logLevelStr, err := parseLogLevel(logMsg.logLevel)
			if err != nil {
				panic(err)
			}
			logStr := fmt.Sprintf("[%s] [%v] [%s:%s:line:%d] %s\n", logMsg.nowStr, logLevelStr, logMsg.funcName, logMsg.fileName, logMsg.lineNum, logMsg.msg)
			fmt.Fprintf(l.logFileObj, logStr)
			if logMsg.logLevel >= ERROR {
				////错误日志按日志文件大小切割，
				if l.needSplit(l.errLogFileObj) {
					newFileObj := l.splitLogFile(l.errLogFileObj)
					if newFileObj == nil {
						panic("新建日志打开日志文件返回为空")
					}
					l.errLogFileObj = newFileObj
				}
				//错误日志按日志时间切割
				//if l.needSplitByHour(l.errLogFileObj){
				//	newFileObj := l.splitLogFileByHour(l.errLogFileObj)
				//	if newFileObj == nil {
				//		panic("新建日志打开日志文件返回为空")
				//	}
				//	l.errLogFileObj = newFileObj
				//}
				fmt.Fprintf(l.errLogFileObj, logStr)
			}
		default:
			//fmt.Println("通道空了，没得取了..")
			time.Sleep(time.Millisecond * 500)
		}
	}
}

// writeToChan 写日志到临时通道中
func (l *FileLogger) writeToChan(logLevel logLevel, formatMsg string, a ...any) {
	if l.enable(logLevel) {
		nowStr := time.Now().Format("2006-01-02 15:04:05")
		funcName, fileName, lineNum := getInfo(3)
		msg := fmt.Sprintf(formatMsg, a...) // 格式化字符串
		logTmp := &logMsg{
			logLevel: logLevel,
			nowStr:   nowStr,
			funcName: funcName,
			fileName: fileName,
			lineNum:  lineNum,
			msg:      msg,
		}
		// 防止通道满了阻塞，使用select , 满了就啥也不干
		select {
		//把日志信息放进队列就不管了，会有另一个groutine去读
		case l.logChan <- logTmp:
		default:
			fmt.Println("日志通道溢出，日志将丢失")
		}

	}

}

// Debug 写入文件
func (l *FileLogger) Debug(formatMsg string, a ...any) {
	l.writeToChan(DEBUG, formatMsg, a...)
}

// Trace 跟踪日志
func (l *FileLogger) Trace(formatMsg string, a ...any) {
	l.writeToChan(TRACE, formatMsg, a...)
}

// Info Info级别日志
func (l *FileLogger) Info(formatMsg string, a ...any) {
	l.writeToChan(INFO, formatMsg, a...)
}

// Warning 级别日志
func (l *FileLogger) Warning(formatMsg string, a ...any) {
	l.writeToChan(WARNING, formatMsg, a...)
}

// Error 错误级别日志
func (l *FileLogger) Error(formatMsg string, a ...any) {
	l.writeToChan(ERROR, formatMsg, a...)
}

// Fatal 灾难级别日志
func (l *FileLogger) Fatal(formatMsg string, a ...any) {
	l.writeToChan(FATAL, formatMsg, a...)
}
