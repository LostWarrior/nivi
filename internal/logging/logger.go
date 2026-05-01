package logging

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"github.com/fatih/color"
        "github.com/mattn/go-isatty"
        "myapp/internal/theme"
)

type Level int

var (
    userC     *color.Color
    assistantC *color.Color
    systemC  *color.Color
)

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

type Field struct {
	Key   string
	Value any
}

type Logger struct {
	mu    sync.Mutex
	out   io.Writer
	level Level
}

func New(level string, out io.Writer) *Logger {
	if out == nil {
		out = io.Discard
	}
	return &Logger{
		out:   out,
		level: parseLevel(level),
	}
}

func Stderr(level string) *Logger {
	return New(level, os.Stderr)
}

func (l *Logger) Debug(message string, fields ...Field) {
	l.log(LevelDebug, message, fields...)
}

func (l *Logger) Info(message string, fields ...Field) {
	l.log(LevelInfo, message, fields...)
}

func (l *Logger) Warn(message string, fields ...Field) {
	l.log(LevelWarn, message, fields...)
}

func (l *Logger) Error(message string, fields ...Field) {
	l.log(LevelError, message, fields...)
}

func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func (l *Logger) log(level Level, message string, fields ...Field) {
	if l == nil || level < l.level {
		return
	}

	var builder strings.Builder
	builder.WriteString(time.Now().UTC().Format(time.RFC3339))
	builder.WriteString(" level=")
	builder.WriteString(level.String())
	builder.WriteString(" msg=")
	builder.WriteString(strconv.Quote(Redact(strings.TrimSpace(message))))

	for _, field := range fields {
		if strings.TrimSpace(field.Key) == "" {
			continue
		}
		builder.WriteByte(' ')
		builder.WriteString(field.Key)
		builder.WriteByte('=')
		builder.WriteString(strconv.Quote(Redact(fmt.Sprint(field.Value))))
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = io.WriteString(l.out, builder.String()+"\n")
}

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	default:
		return "error"
	}
}

func parseLevel(level string) Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return LevelDebug
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}
