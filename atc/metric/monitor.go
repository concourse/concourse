package metric

import (
	"code.cloudfoundry.org/lager"
)

type Metric interface {
	Events() []Event
	Name()   string
}

type Monitor interface {
	lager.Logger
	Measure(Metric)
}

type monitor struct {
	lager.Logger
}

func NewMonitor(logger lager.Logger) Monitor {
	return &monitor{logger}
}

func (mon *monitor) Measure(m Metric) {
	for _, event := range m.Events() {
		emit(mon.Session(m.Name()), event)
	}
}
