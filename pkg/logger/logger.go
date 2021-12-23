package logger

import (
	"cloud.google.com/go/logging"
	"context"
	"log"
	"os"
)

// Interface is the interface all loggers have to implement
type Interface interface {
	Error(message string, err error)
	Warning(message string, err error)
	Info(message string)
	Debug(message string)
	Fatal(err error)
}

// Logger is the logger type itself
type Logger struct {
}

// Error is for throwing a log message with status Error
func (l Logger) Error(message string, err error) {
	log.Printf("[ERROR] %s: %+v\n", message, err)
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
	log.Fatalf("[FATAL] %+v\n", err)
}

// Warning is for throwing a log message with status Warning
func (l Logger) Warning(message string, err error) {
	log.Printf("[WARNING] %s: %+v\n", message, err)
}

// GoogleCloudLogger is for production use on the google cloud
type GoogleCloudLogger struct {
	standardLogger Interface
	logger         *logging.Logger
}

// NewGoogleCloudLogger Constructor for GoogleCloudLogger. It uses the GOOGLE_CLOUD_PROJECT env variable
func NewGoogleCloudLogger() Interface {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")

	ctx := context.Background()

	// Creates a client.
	client, err := logging.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create client: %+v", err)
	}

	// Sets the name of the log to write to.
	logName := "timeliness"

	return GoogleCloudLogger{
		logger:         client.Logger(logName),
		standardLogger: Logger{},
	}
}

// Error severity
func (g GoogleCloudLogger) Error(message string, err error) {
	g.logger.StandardLogger(logging.Error).Printf("[ERROR] %s: %+v\n", message, err)
}

// Warning Severity
func (g GoogleCloudLogger) Warning(message string, err error) {
	g.logger.StandardLogger(logging.Warning).Printf("[WARNING] %s: %+v\n", message, err)
}

// Info severity
func (g GoogleCloudLogger) Info(message string) {
	g.logger.StandardLogger(logging.Info).Printf("[Info] %s\n", message)
}

// Debug severity
func (g GoogleCloudLogger) Debug(message string) {
	g.logger.StandardLogger(logging.Debug).Printf("[DEBUG] %s\n", message)
}

// Fatal severity
func (g GoogleCloudLogger) Fatal(err error) {
	g.logger.StandardLogger(logging.Critical).Printf("[FATAL] %+v\n", err)
}
