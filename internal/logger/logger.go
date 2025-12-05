// Package logger provides structured logging utilities.
package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents logging level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger defines the logging interface.
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	WithContext(key string, value interface{}) Logger
	SetLevel(level Level)
}

// SimpleLogger is a basic structured logger implementation.
type SimpleLogger struct {
	mu      sync.Mutex
	name    string
	level   Level
	out     io.Writer
	context map[string]interface{}
}

// New creates a new SimpleLogger with the given name.
func New(name string) *SimpleLogger {
	return &SimpleLogger{
		name:    name,
		level:   LevelInfo,
		out:     os.Stdout,
		context: make(map[string]interface{}),
	}
}

// SetOutput sets the output destination.
func (l *SimpleLogger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = w
}

// SetLevel sets the minimum logging level.
func (l *SimpleLogger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// WithContext returns a new logger with additional context.
func (l *SimpleLogger) WithContext(key string, value interface{}) Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newContext := make(map[string]interface{})
	for k, v := range l.context {
		newContext[k] = v
	}
	newContext[key] = value

	return &SimpleLogger{
		name:    l.name,
		level:   l.level,
		out:     l.out,
		context: newContext,
	}
}

func (l *SimpleLogger) log(level Level, msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Build context string.
	contextStr := ""
	for k, v := range l.context {
		contextStr += fmt.Sprintf(" %s=%v", k, v)
	}

	// Build args string.
	argsStr := ""
	for i := 0; i < len(args)-1; i += 2 {
		if i+1 < len(args) {
			argsStr += fmt.Sprintf(" %v=%v", args[i], args[i+1])
		}
	}

	fmt.Fprintf(l.out, "[%s] %s [%s]%s %s%s\n",
		timestamp, level.String(), l.name, contextStr, msg, argsStr)
}

// Debug logs a debug message.
func (l *SimpleLogger) Debug(msg string, args ...interface{}) {
	l.log(LevelDebug, msg, args...)
}

// Info logs an info message.
func (l *SimpleLogger) Info(msg string, args ...interface{}) {
	l.log(LevelInfo, msg, args...)
}

// Warn logs a warning message.
func (l *SimpleLogger) Warn(msg string, args ...interface{}) {
	l.log(LevelWarn, msg, args...)
}

// Error logs an error message.
func (l *SimpleLogger) Error(msg string, args ...interface{}) {
	l.log(LevelError, msg, args...)
}

// Default is the default global logger.
var Default = New("git")

// Debug logs a debug message using the default logger.
func Debug(msg string, args ...interface{}) {
	Default.Debug(msg, args...)
}

// Info logs an info message using the default logger.
func Info(msg string, args ...interface{}) {
	Default.Info(msg, args...)
}

// Warn logs a warning message using the default logger.
func Warn(msg string, args ...interface{}) {
	Default.Warn(msg, args...)
}

// Error logs an error message using the default logger.
func Error(msg string, args ...interface{}) {
	Default.Error(msg, args...)
}
