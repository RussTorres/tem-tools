package logger

const (
	// FatalLevel constant for fatal level logging
	FatalLevel int = iota
	// ErrorLevel constant for error level logging
	ErrorLevel
	// InfoLevel constant for info level logging
	InfoLevel
	// DebugLevel constant for debug level logging
	DebugLevel
	// TraceLevel constant for trace level logging
	TraceLevel
)

// Logger responsible for application logging.
type Logger interface {
	Logf(level int, format string, args ...interface{})
	Log(level int, args ...interface{})
}

var currentLogger Logger

// Debugf - log message at debug level
func Debugf(format string, args ...interface{}) {
	currentLogger.Logf(DebugLevel, format, args...)
}

// Infof - log message at info level
func Infof(format string, args ...interface{}) {
	currentLogger.Logf(InfoLevel, format, args...)
}

// Errorf - log message at error level
func Errorf(format string, args ...interface{}) {
	currentLogger.Logf(ErrorLevel, format, args...)
}

// Logf - log message at given level
func Logf(level int, format string, args ...interface{}) {
	currentLogger.Logf(level, format, args...)
}

// Printf - print message irrespective of the log level
func Printf(format string, args ...interface{}) {
	currentLogger.Logf(InfoLevel, format, args...)
}

// Fatalf - log message and exit
func Fatalf(format string, args ...interface{}) {
	currentLogger.Logf(FatalLevel, format, args...)
}

// Debug - log message at debug level
func Debug(args ...interface{}) {
	currentLogger.Log(DebugLevel, args...)
}

// Info - log message at info level
func Info(args ...interface{}) {
	currentLogger.Log(InfoLevel, args...)
}

// Error - log message at error level
func Error(args ...interface{}) {
	currentLogger.Log(ErrorLevel, args...)
}

// Log - log message at given level
func Log(level int, args ...interface{}) {
	currentLogger.Log(level, args...)
}

// Print - print message irrespective of the log level
func Print(args ...interface{}) {
	currentLogger.Log(InfoLevel, args...)
}

// Fatal - log message and exit
func Fatal(args ...interface{}) {
	currentLogger.Log(FatalLevel, args...)
}
