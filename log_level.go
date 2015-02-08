package atc

type LogLevel string

const (
	LogLevelInvalid LogLevel = ""
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelError   LogLevel = "error"
	LogLevelFatal   LogLevel = "fatal"
)
