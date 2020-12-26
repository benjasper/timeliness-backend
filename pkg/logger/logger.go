package logger

import "log"

// Interface is the interface all loggers have to implement
type Interface interface {
	Error(message string, err error)
	Info(message string)
	Debug(message string)
	Fatal(err error)
}

// Logger is the logger type itself
type Logger struct {
}

// Error is for throwing a log message with status Error
func (l Logger) Error(message string, err error) {
	log.Printf("[ERROR] %s: %v\n", message, err)
}

// Info is for throwing a log message with status Info
func (l Logger) Info(message string) {
	log.Printf("[INFO] %s\n", message)
}

// Debug is for throwing a log message with status Debug
func (l Logger) Debug(message string) {
	log.Printf("[DEBUG] %s\n", message)
}

// Fatal is for throwing a log message with status Fatal
func (l Logger) Fatal(err error) {
	log.Fatalf("[FATAL] %v\n", err)
}
