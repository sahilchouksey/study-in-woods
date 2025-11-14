package utils

import (
	"log"
	"os"
	"runtime"
	"time"
)

type Logger struct {
	logger *log.Logger
}

func NewLogger() *Logger {
	file, err := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	return &Logger{
		logger: log.New(file, "", 0),
	}
}

func (l *Logger) Log(message string) {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "unknown"
		line = 0
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	l.logger.Printf("[%s] %s:%d %s\n", timestamp, file, line, message)
}
