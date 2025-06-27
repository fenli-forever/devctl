package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
)

type Logger struct {
	level  LogLevel
	logger *log.Logger
	file   *os.File
}

func NewLogger(level LogLevel, filename string) (*Logger, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	return &Logger{
		level:  level,
		logger: log.New(file, "", log.Ldate|log.Ltime),
		file:   file,
	}, nil
}

func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

func (l *Logger) log(level LogLevel, format string, v ...interface{}) {
	if level < l.level {
		return
	}

	_, file, line, _ := runtime.Caller(2)
	msg := fmt.Sprintf(format, v...)
	l.logger.Printf("[%s] %s:%d: %s", level.String(), filepath.Base(file), line, msg)
}

func (l *Logger) Debug(format string, v ...interface{}) {
	l.log(DEBUG, format, v...)
}

func (l *Logger) Info(format string, v ...interface{}) {
	l.log(INFO, format, v...)
}

func (l *Logger) Warning(format string, v ...interface{}) {
	l.log(WARNING, format, v...)
}

func (l *Logger) Error(format string, v ...interface{}) {
	l.log(ERROR, format, v...)
}

func (l *Logger) RotateLog(maxSize int64, maxFiles int) error {
	info, err := l.file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get log file info: %v", err)
	}

	if info.Size() < maxSize {
		return nil
	}

	l.file.Close()

	// Rename current log file
	now := time.Now().Format("20060102-150405")
	newName := fmt.Sprintf("%s.%s", l.file.Name(), now)
	err = os.Rename(l.file.Name(), newName)
	if err != nil {
		return fmt.Errorf("failed to rename log file: %v", err)
	}

	// Open new log file
	file, err := os.OpenFile(l.file.Name(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open new log file: %v", err)
	}

	l.file = file
	l.logger = log.New(file, "", log.Ldate|log.Ltime)

	// Remove old log files if exceeding maxFiles
	pattern := fmt.Sprintf("%s.*", l.file.Name())
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to list old log files: %v", err)
	}

	if len(matches) > maxFiles {
		for _, f := range matches[:len(matches)-maxFiles] {
			os.Remove(f)
		}
	}

	return nil
}

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
