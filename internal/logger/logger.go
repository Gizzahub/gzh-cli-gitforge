// Package logger provides structured logging for gitforge operations.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// Level represents log level
type Level int

const (
	// DebugLevel for debugging information
	DebugLevel Level = iota
	// InfoLevel for general information
	InfoLevel
	// WarnLevel for warnings
	WarnLevel
	// ErrorLevel for errors
	ErrorLevel
)

var levelNames = map[Level]string{
	DebugLevel: "DEBUG",
	InfoLevel:  "INFO",
	WarnLevel:  "WARN",
	ErrorLevel: "ERROR",
}

// Logger provides structured logging with levels
type Logger struct {
	component string
	level     Level
	mu        sync.Mutex
	output    io.Writer
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// init initializes the default logger
func init() {
	defaultLogger = New("default")
}

// New creates a new logger for a specific component
func New(component string) *Logger {
	return &Logger{
		component: component,
		level:     InfoLevel,
		output:    os.Stderr,
	}
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutput sets the output writer
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// log writes a log message with the given level
func (l *Logger) log(level Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelName := levelNames[level]
	message := fmt.Sprintf(format, args...)

	logLine := fmt.Sprintf("[%s] %s [%s] %s\n", timestamp, levelName, l.component, message)
	fmt.Fprint(l.output, logLine)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DebugLevel, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(InfoLevel, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WarnLevel, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ErrorLevel, format, args...)
}

// WithField returns a logger with an additional field
func (l *Logger) WithField(key, value string) *Logger {
	return &Logger{
		component: fmt.Sprintf("%s:%s=%s", l.component, key, value),
		level:     l.level,
		output:    l.output,
	}
}

// Global logging functions using the default logger

// SetGlobalLevel sets the minimum log level for the default logger
func SetGlobalLevel(level Level) {
	defaultLogger.SetLevel(level)
}

// SetGlobalOutput sets the output writer for the default logger
func SetGlobalOutput(w io.Writer) {
	defaultLogger.SetOutput(w)
}

// Debug logs a debug message using the default logger
func Debug(format string, args ...interface{}) {
	defaultLogger.Debug(format, args...)
}

// Info logs an info message using the default logger
func Info(format string, args ...interface{}) {
	defaultLogger.Info(format, args...)
}

// Warn logs a warning message using the default logger
func Warn(format string, args ...interface{}) {
	defaultLogger.Warn(format, args...)
}

// Error logs an error message using the default logger
func Error(format string, args ...interface{}) {
	defaultLogger.Error(format, args...)
}

// Fatal logs an error message and exits
func Fatal(format string, args ...interface{}) {
	defaultLogger.Error(format, args...)
	os.Exit(1)
}

// Deprecated: Use New() instead. Kept for compatibility.
func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
