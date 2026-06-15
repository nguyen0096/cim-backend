package log

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	Logger *logrus.Logger
)

// DefaultConfig provides a fallback configuration if Logger is not initialized
var DefaultConfig = Config{
	LogLevel: logrus.ErrorLevel,
}

type Config struct {
	LogLevel logrus.Level
	// Environment controls the OUTPUT FORMAT only (JSON for non-local, text for
	// local/development). It must NEVER gate stack-trace content: stacks are
	// emitted in every environment.
	Environment string
}

func init() {
	Logger = MustInit(DefaultConfig)
}

// Init creates and initializes a new logger with the given config
func Init(cfg Config) (*logrus.Logger, error) {
	logger := logrus.New()
	logger.SetLevel(cfg.LogLevel)

	// Include the calling method as a field. This affects all log entries in
	// every environment.
	logger.SetReportCaller(true)

	// Formatter selection keys off the environment for FORMAT ONLY (structured
	// JSON for deployed environments, human-readable text for local). This does
	// NOT gate stack-trace content; the "stack_trace" field is emitted in both.
	if isLocalEnvironment(cfg.Environment) {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	} else {
		logger.SetFormatter(&logrus.JSONFormatter{})
	}

	Logger = logger
	return logger, nil
}

// isLocalEnvironment reports whether the environment is a local/development one
// (text formatter). Anything else (e.g. staging, production) uses JSON.
func isLocalEnvironment(env string) bool {
	switch env {
	case "", "local", "development", "dev", "test":
		return true
	default:
		return false
	}
}

// MustInit creates and initializes a new logger, panicking on error
func MustInit(cfg Config) *logrus.Logger {
	logger, err := Init(cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	return logger
}

// Basic logging methods

// Debug logs a message at level Debug
func Debug(args ...interface{}) {
	Logger.Debug(args...)
}

// Info logs a message at level Info
func Info(args ...interface{}) {
	Logger.Info(args...)
}

// Warn logs a message at level Warn
func Warn(args ...interface{}) {
	Logger.Warn(args...)
}

// Error logs a message at level Error
func Error(args ...interface{}) {
	Logger.Error(args...)
}

// Fatal logs a message at level Fatal then the process will exit with status 1
func Fatal(args ...interface{}) {
	Logger.Fatal(args...)
}

// Panic logs a message at level Panic then panics
func Panic(args ...interface{}) {
	Logger.Panic(args...)
}

// Trace logs a message at level Trace
func Trace(args ...interface{}) {
	Logger.Trace(args...)
}

// Print logs a message at level Info
func Print(args ...interface{}) {
	Logger.Print(args...)
}

// Formatted logging methods

// Debugf logs a formatted message at level Debug
func Debugf(format string, args ...interface{}) {
	Logger.Debugf(format, args...)
}

// Infof logs a formatted message at level Info
func Infof(format string, args ...interface{}) {
	Logger.Infof(format, args...)
}

// Warnf logs a formatted message at level Warn
func Warnf(format string, args ...interface{}) {
	Logger.Warnf(format, args...)
}

// Errorf logs a formatted message at level Error
func Errorf(format string, args ...interface{}) {
	Logger.Errorf(format, args...)
}

// Fatalf logs a formatted message at level Fatal then the process will exit with status 1
func Fatalf(format string, args ...interface{}) {
	Logger.Fatalf(format, args...)
}

// Panicf logs a formatted message at level Panic then panics
func Panicf(format string, args ...interface{}) {
	Logger.Panicf(format, args...)
}

// Tracef logs a formatted message at level Trace
func Tracef(format string, args ...interface{}) {
	Logger.Tracef(format, args...)
}

// Printf logs a formatted message at level Info
func Printf(format string, args ...interface{}) {
	Logger.Printf(format, args...)
}

// Line logging methods

// Debugln logs a message at level Debug
func Debugln(args ...interface{}) {
	Logger.Debugln(args...)
}

// Infoln logs a message at level Info
func Infoln(args ...interface{}) {
	Logger.Infoln(args...)
}

// Warnln logs a message at level Warn
func Warnln(args ...interface{}) {
	Logger.Warnln(args...)
}

// Errorln logs a message at level Error
func Errorln(args ...interface{}) {
	Logger.Errorln(args...)
}

// Fatalln logs a message at level Fatal then the process will exit with status 1
func Fatalln(args ...interface{}) {
	Logger.Fatalln(args...)
}

// Panicln logs a message at level Panic then panics
func Panicln(args ...interface{}) {
	Logger.Panicln(args...)
}

// Traceln logs a message at level Trace
func Traceln(args ...interface{}) {
	Logger.Traceln(args...)
}

// Println logs a message at level Info
func Println(args ...interface{}) {
	Logger.Println(args...)
}

// Structured logging methods

// WithField creates an entry with a single field
func WithField(key string, value interface{}) *logrus.Entry {
	return Logger.WithField(key, value)
}

// WithFields creates an entry with multiple fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Logger.WithFields(fields)
}

// WithError creates an entry with an error field
func WithError(err error) *logrus.Entry {
	return Logger.WithError(err)
}

// WithContext creates an entry with a context
func WithContext(ctx context.Context) *logrus.Entry {
	return Logger.WithContext(ctx)
}

// WithTime creates an entry with a specific timestamp
func WithTime(t time.Time) *logrus.Entry {
	return Logger.WithTime(t)
}

// Configuration methods

// SetLevel sets the logger level
func SetLevel(level logrus.Level) {
	Logger.SetLevel(level)
}

// GetLevel returns the logger level
func GetLevel() logrus.Level {
	return Logger.GetLevel()
}

// SetFormatter sets the logger formatter
func SetFormatter(formatter logrus.Formatter) {
	Logger.SetFormatter(formatter)
}

// SetOutput sets the logger output
func SetOutput(output io.Writer) {
	Logger.SetOutput(output)
}

// SetReportCaller sets whether the logger should include the calling method as a field
func SetReportCaller(reportCaller bool) {
	Logger.SetReportCaller(reportCaller)
}

// IsLevelEnabled checks if the logger will log at the given level
func IsLevelEnabled(level logrus.Level) bool {
	return Logger.IsLevelEnabled(level)
}

// Advanced logging methods

// Log logs a message at the given level
func Log(level logrus.Level, args ...interface{}) {
	Logger.Log(level, args...)
}

// Logf logs a formatted message at the given level
func Logf(level logrus.Level, format string, args ...interface{}) {
	Logger.Logf(level, format, args...)
}

// Logln logs a message at the given level
func Logln(level logrus.Level, args ...interface{}) {
	Logger.Logln(level, args...)
}
