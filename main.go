package gologger

import (
	"fmt"
	"log/syslog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"
)

type Log struct {
	logChan      chan string
	syslogWriter *syslog.Writer
	fileWriter   *os.File

	SyslogTag     string
	Priority      syslog.Priority
	SendToStdout  bool
	SendToSyslog  bool
	SendToLogfile bool
}

func (l *Log) daemon() {
	runtime.LockOSThread()
	_, _, oldDate := time.Now().Date()

	for {
		message := <-l.logChan
		messageWithTimestamp := time.Now().Format("15:04:05.0000") + message

		if l.SendToStdout {
			fmt.Print(messageWithTimestamp)
		}

		if l.SendToSyslog {
			if l.syslogWriter == nil {
				syslogWriter, err := syslog.New(syslog.LOG_INFO, l.SyslogTag)
				if err != nil {
					l.ERR(err, "Error creating syslog")
					return
				}
				l.syslogWriter = syslogWriter
			}
			_, err := l.syslogWriter.Write([]byte(message))
			if err != nil {
				l.ERR(err, "Error writing to syslog")
				return
			}
		}

		if l.SendToLogfile {
			if l.fileWriter == nil {
				l.newFile()
			}
			_, _, newDate := time.Now().Date()
			if newDate != oldDate {
				l.newFile()
				oldDate = newDate
			}

			_, err := l.fileWriter.Write([]byte(messageWithTimestamp))
			if err != nil {
				l.ERR(err, "Error writing to logfile")
				return
			}
		}
	}
}

func (l *Log) newFile() {
	if l.fileWriter != nil {
		err := l.fileWriter.Close()
		if err != nil {
			l.ERR(err, "Error closing logfile")
			return
		}
	}

	fileName := filepath.Base(os.Args[0]) + "_" + time.Now().Format("2006-01-02") + ".log"
	fileWriter, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		l.ERR(err, "Error creating logfile")
		return
	}
	l.fileWriter = fileWriter
}

func (l *Log) anyErrToString(e interface{}, prompt string) string {
	switch t := e.(type) {
	case error:
		return fmt.Sprintf("%s err{%s}", prompt, e.(error).Error())
	case string:
		return fmt.Sprintf("%s err{%s}", prompt, e.(string))
	case byte, int:
		return fmt.Sprintf("%s RC:%02d", prompt, e.(byte))
	default:
		return fmt.Sprintf("%s ???{type(%v)=%v}", prompt, t, e)
	}
}

func (l *Log) Log(stackTraceDepth int, level byte, message string) {
	var funcName string

	// 2 + stackTraceDepth because first layer is Log(), second layer is ERR/INF/DBG()
	pc, _, line, ok := runtime.Caller(2 + stackTraceDepth)
	if !ok {
		funcName = "<nf>"
	} else {
		funcName = runtime.FuncForPC(pc).Name()

		// print struct func as regular func
		// for example: main.(*Test).exampleFunc() -> Test.exampleFunc()
		// match will be array of {"(*Test).exampleFunc", "Test", "exampleFunc"}
		match := regexp.MustCompile(`\(\*([0-z_]+)\)\.([0-z_\(\)]+)$`).FindStringSubmatch(funcName)
		if match != nil {
			funcName = match[1]
			if len(match) > 2 {
				funcName += "." + match[len(match)-1]
			}
		}
	}

	s := fmt.Sprintf("|%c|%s():%d %s\n", level, funcName, line, message)

	l.logChan <- s
}

func (l *Log) ERR(e interface{}, prompt string, v ...interface{}) {
	if l.Priority < syslog.LOG_ERR {
		return
	}
	if v != nil {
		prompt = fmt.Sprintf(prompt, v...)
	}

	l.Log(0, 'E', l.anyErrToString(e, prompt))
}

func (l *Log) WRN(prompt string, v ...interface{}) {
	if l.Priority < syslog.LOG_WARNING {
		return
	}
	if v != nil {
		prompt = fmt.Sprintf(prompt, v...)
	}
	l.Log(0, 'W', prompt)
}

func (l *Log) INF(prompt string, v ...interface{}) {
	if l.Priority < syslog.LOG_INFO {
		return
	}
	if v != nil {
		prompt = fmt.Sprintf(prompt, v...)
	}
	l.Log(0, 'I', prompt)
}

func (l *Log) DBG(prompt string, v ...interface{}) {
	if l.Priority < syslog.LOG_DEBUG {
		return
	}
	if v != nil {
		prompt = fmt.Sprintf(prompt, v...)
	}
	l.Log(0, 'D', prompt)
}

func (l *Log) Close() {
	lenLogChan := len(l.logChan)
	for lenLogChan > 0 {
		time.Sleep(100 * time.Millisecond)
		lenLogChan = len(l.logChan)
	}
}

var L *Log

func init() {
	L = &Log{
		logChan:       make(chan string, 1000),
		SendToStdout:  true, // The logger prints to stdout as a default, though can be easily changed.
		SendToSyslog:  false,
		SendToLogfile: false,
		Priority:      syslog.LOG_DEBUG,
		SyslogTag:     "GOLOGGER",
	}

	go L.daemon()
}
