package metric

import (
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/db"
	flags "github.com/jessevdk/go-flags"
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
	Description() string
	IsConfigured() bool
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
	ChecksDeleted     Counter

	JobsScheduled  Counter
	JobsScheduling Gauge

	BuildsStarted Counter
	BuildsRunning Gauge

	TasksWaiting map[TasksWaitingLabels]*Gauge

	ChecksFinishedWithError   Counter
	ChecksFinishedWithSuccess Counter
	ChecksQueueSize           Gauge
	ChecksStarted             Counter
	ChecksEnqueued            Counter

	ConcurrentRequests         map[string]*Gauge
	ConcurrentRequestsLimitHit map[string]*Counter
}

var Metrics = NewMonitor()

func NewMonitor() *Monitor {
	return &Monitor{
		TasksWaiting:               map[TasksWaitingLabels]*Gauge{},
		ConcurrentRequests:         map[string]*Gauge{},
		ConcurrentRequestsLimitHit: map[string]*Counter{},
	}
}

func (m *Monitor) RegisterEmitter(factory EmitterFactory) {
	m.emitterFactories = append(m.emitterFactories, factory)
}

func (m *Monitor) WireEmitters(group *flags.Group) {
	for _, factory := range m.emitterFactories {
		_, err := group.AddGroup(fmt.Sprintf("Metric Emitter (%s)", factory.Description()), "", factory)
		if err != nil {
			panic(err)
		}
	}
}

type eventEmission struct {
	event  Event
	logger lager.Logger
}

func (m *Monitor) Initialize(logger lager.Logger, host string, attributes map[string]string, bufferSize uint32) error {
	logger.Debug("metric-initialize", lager.Data{
		"host":        host,
		"attributes":  attributes,
		"buffer-size": bufferSize,
	})

	var (
		emitterDescriptions []string
		err                 error
	)

	for _, factory := range m.emitterFactories {
		if factory.IsConfigured() {
			emitterDescriptions = append(emitterDescriptions, factory.Description())
		}
	}
	if len(emitterDescriptions) > 1 {
		return fmt.Errorf("Multiple emitters configured: %s", strings.Join(emitterDescriptions, ", "))
	}

	var emitter Emitter

	for _, factory := range m.emitterFactories {
		if factory.IsConfigured() {
			emitter, err = factory.NewEmitter()
			if err != nil {
				return err
			}
		}
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
