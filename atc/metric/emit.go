package metric

import (
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/db"
)

type Event struct {
	Name       string
	Value      float64
	Attributes map[string]string
	Host       string
	Time       time.Time
}

//go:generate counterfeiter . Emitter
type Emitter interface {
	Emit(lager.Logger, Event)
}

//go:generate counterfeiter . EmitterFactory
type EmitterFactory interface {
	ID() string
	Description() string
	Validate() error
	NewEmitter() (Emitter, error)
}

type Monitor struct {
	emitter          Emitter
	eventHost        string
	eventAttributes  map[string]string
	emissions        chan eventEmission
	emitterFactories []EmitterFactory

	Databases       []db.Conn
	DatabaseQueries Counter

	ContainersCreated Counter
	VolumesCreated    Counter

	FailedContainers Counter
	FailedVolumes    Counter

	ContainersDeleted Counter
	VolumesDeleted    Counter

	JobsScheduled  Counter
	JobsScheduling Gauge

	BuildsStarted Counter
	BuildsRunning Gauge

	CheckBuildsStarted Counter
	CheckBuildsRunning Gauge

	StepsWaiting map[StepsWaitingLabels]*Gauge

	// TODO: deprecate, replaced with CheckBuildFinished
	ChecksFinishedWithError   Counter
	ChecksFinishedWithSuccess Counter

	// TODO: deprecate, replaced with CheckBuildsStarted and CheckBuildStarted
	ChecksStarted Counter

	ChecksEnqueued Counter

	ConcurrentRequests         map[string]*Gauge
	ConcurrentRequestsLimitHit map[string]*Counter

	VolumesStreamed Counter
}

var Metrics = NewMonitor()

func NewMonitor() *Monitor {
	return &Monitor{
		StepsWaiting:               map[StepsWaitingLabels]*Gauge{},
		ConcurrentRequests:         map[string]*Gauge{},
		ConcurrentRequestsLimitHit: map[string]*Counter{},
	}
}

type eventEmission struct {
	event  Event
	logger lager.Logger
}

func (m *Monitor) Initialize(logger lager.Logger, factory EmitterFactory, host string, attributes map[string]string, bufferSize uint32) error {
	logger.Debug("metric-initialize", lager.Data{
		"host":        host,
		"attributes":  attributes,
		"buffer-size": bufferSize,
	})

	emitter, err := factory.NewEmitter()
	if err != nil {
		return err
	}

	if emitter == nil {
		return nil
	}

	m.emitter = emitter
	m.eventHost = host
	m.eventAttributes = attributes
	m.emissions = make(chan eventEmission, int(bufferSize))

	go m.emitLoop()

	return nil
}

func (m *Monitor) emit(logger lager.Logger, event Event) {
	if m.emitter == nil {
		return
	}

	event.Host = m.eventHost
	event.Time = time.Now()

	mergedAttributes := map[string]string{}
	for k, v := range m.eventAttributes {
		mergedAttributes[k] = v
	}

	if event.Attributes != nil {
		for k, v := range event.Attributes {
			mergedAttributes[k] = v
		}
	}

	event.Attributes = mergedAttributes

	select {
	case m.emissions <- eventEmission{logger: logger, event: event}:
	default:
		logger.Error("queue-full", nil)
	}
}

func (m *Monitor) emitLoop() {
	for emission := range m.emissions {
		m.emitter.Emit(emission.logger.Session("emit"), emission.event)
	}
}
