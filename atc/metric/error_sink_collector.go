package metric

import (
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
)

type ErrorSinkCollector struct {
	logger  lager.Logger
	monitor *Monitor
}

func NewErrorSinkCollector(logger lager.Logger, m *Monitor) ErrorSinkCollector {
	return ErrorSinkCollector{
		logger:  logger,
		monitor: m,
	}
}

func (c *ErrorSinkCollector) Log(f lager.LogFormat) {
	if f.LogLevel != lager.ERROR {
		return
	}

	if errors.Cause(f.Error) == ErrFailedToEmit {
		return
	}

	ErrorLog{
		Value:   1,
		Message: f.Message,
	}.Emit(c.logger, c.monitor)
}
