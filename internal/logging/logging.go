package logging

import (
	"log"
	"os"
)

type Logger struct {
	info  *log.Logger
	err   *log.Logger
	level string
}

func New(level string) *Logger {
	return &Logger{
		info:  log.New(os.Stdout, "[INFO] ", log.LstdFlags),
		err:   log.New(os.Stderr, "[ERROR] ", log.LstdFlags),
		level: level,
	}
}

func (l *Logger) Info(format string, v ...interface{}) {
	if l.level == "debug" || l.level == "info" {
		l.info.Printf(format, v...)
	}
}

func (l *Logger) Error(format string, v ...interface{}) {
	l.err.Printf(format, v...)
}
