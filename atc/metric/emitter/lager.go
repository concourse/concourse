package emitter

import (
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/metric"
)

type LagerEmitter struct{}

type LagerConfig struct {
	Enabled bool `yaml:"enable,omitempty" env:"CONCOURSE_LAGER_ENABLE,CONCOURSE_EMIT_TO_LOGS"`
}

func (config *LagerConfig) Description() string { return "Lager" }
func (config *LagerConfig) IsConfigured() bool  { return config.Enabled }

func (config *LagerConfig) NewEmitter() (metric.Emitter, error) {
	return &LagerEmitter{}, nil
}

func (emitter *LagerEmitter) Emit(logger lager.Logger, event metric.Event) {
	data := lager.Data{
		"name":  event.Name,
		"value": event.Value,
	}

	for k, v := range event.Attributes {
		// normalize on foo-bar rather than foo_bar
		lagerKey := strings.Replace(k, "_", "-", -1)
		data[lagerKey] = v
	}

	logger.Debug("event", data)
}
