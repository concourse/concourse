package metric

import (
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/lager/v3"

	"github.com/concourse/concourse/atc/db"
	flags "github.com/jessevdk/go-flags"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

type Event struct {
	Name       string
	Value      float64
	Attributes map[string]string
	Host       string
	Time       time.Time
}

//counterfeiter:generate . Emitter
type Emitter interface {
	Emit(lager.Logger, Event)
}

//counterfeiter:generate . EmitterFactory
type EmitterFactory interface {
	Description() string
	IsConfigured() bool
	NewEmitter(map[string]string) (Emitter, error)
}

type Monitor struct {
	emitter          Emitter
	eventHost        string
	eventAttributes  map[string]string
	emissions        chan eventEmission
	emitterFactories []EmitterFactory

	Databases       []db.DbConn
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

	JobStatuses  map[JobStatusLabels]*Gauge
	StepsWaiting map[StepsWaitingLabels]*Gauge

	// When global resource is not enabled, ChecksStarted should equal to CheckBuildsStarted.
	// But with global resource enabled, ChecksStarted measures how many checks really run.
	// For example, there are 10 resources having exact same config, so they belong to the same
	// resource configure scope. In each check period, 10 check builds will be created,
	// CheckBuildsStarted should be 10. But only 1 check build should run real check, rest 9 check
	// builds should reuse the first check's result, thus ChecksStarted will be 1.
	// The bigger diff between ChecksStarted and CheckBuildsStarted, the more global resource benefits.
	ChecksStarted Counter

	// ChecksFinishedWithError+ChecksFinishedWithSuccess should equal to ChecksStarted.
	ChecksFinishedWithError   Counter
	ChecksFinishedWithSuccess Counter

	ChecksEnqueued Counter

	ConcurrentRequests         map[string]*Gauge
	ConcurrentRequestsLimitHit map[string]*Counter

	VolumesStreamed Counter

	GetStepCacheHits       Counter
	StreamedResourceCaches Counter
}

var Metrics = NewMonitor()

func NewMonitor() *Monitor {
	return &Monitor{
		StepsWaiting:               map[StepsWaitingLabels]*Gauge{},
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
		return fmt.Errorf("multiple emitters configured: %s", strings.Join(emitterDescriptions, ", "))
	}

	var emitter Emitter

	for _, factory := range m.emitterFactories {
		if factory.IsConfigured() {
			emitter, err = factory.NewEmitter(attributes)
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
