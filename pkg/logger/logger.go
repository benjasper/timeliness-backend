package logger

import "log"

type Interface interface {
	Error(message string, err error)
	Info(message string)
	Debug(message string)
	Fatal(err error)
}

type Logger struct {
}

func (l Logger) Error(message string, err error) {
	log.Printf("[ERROR] %s: %v\n", message, err)
}

func (l Logger) Info(message string) {
	log.Printf("[INFO] %s\n", message)
}

func (l Logger) Debug(message string) {
	log.Printf("[DEBUG] %s\n", message)
}

func (l Logger) Fatal(err error) {
	log.Fatalf("[FATAL] %v\n", err)
}
