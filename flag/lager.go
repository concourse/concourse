package flag

import (
	"fmt"
	"io"
	"os"

	"code.cloudfoundry.org/lager"
)

const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
)

var ValidLogLevels = []string{
	LogLevelDebug,
	LogLevelInfo,
	LogLevelError,
	LogLevelFatal,
}

type Lager struct {
	LogLevel   string `yaml:"log_level,omitempty" validate:"log_level" env:"CONCOURSE_WORKER_GATEWAY_LOG_LEVEL,CONCOURSE_TSA_LOG_LEVEL"`
	writerSink io.Writer
}

func (f *Lager) SetWriterSink(writer io.Writer) {
	f.writerSink = writer
}

func (f Lager) Logger(component string) (lager.Logger, *lager.ReconfigurableSink) {
	var minLagerLogLevel lager.LogLevel
	switch f.LogLevel {
	case LogLevelDebug:
		minLagerLogLevel = lager.DEBUG
	case LogLevelInfo:
		minLagerLogLevel = lager.INFO
	case LogLevelError:
		minLagerLogLevel = lager.ERROR
	case LogLevelFatal:
		minLagerLogLevel = lager.FATAL
	default:
		panic(fmt.Sprintf("unknown log level: %s", f.LogLevel))
	}

	logger := lager.NewLogger(component)

	if f.writerSink == nil {
		f.writerSink = os.Stdout
	}
	sink := lager.NewReconfigurableSink(lager.NewPrettySink(f.writerSink, lager.DEBUG), minLagerLogLevel)

	logger.RegisterSink(sink)

	return logger, sink
}
