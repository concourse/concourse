package emitter

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/DataDog/datadog-go/statsd"
	"github.com/concourse/concourse/atc/metric"
	"github.com/pkg/errors"
)

type DogstatsdEmitter struct {
	client *statsd.Client
}

type DogstatsDBConfig struct {
	Host   string `long:"datadog-agent-host" description:"Datadog agent host to expose dogstatsd metrics"`
	Port   string `long:"datadog-agent-port" description:"Datadog agent port to expose dogstatsd metrics"`
	Prefix string `long:"datadog-prefix" description:"Prefix for all metrics to easily find them in Datadog"`
}

func init() {
	metric.Metrics.RegisterEmitter(&DogstatsDBConfig{})
}

func (config *DogstatsDBConfig) Description() string { return "Datadog" }

func (config *DogstatsDBConfig) IsConfigured() bool { return config.Host != "" && config.Port != "" }

func (config *DogstatsDBConfig) NewEmitter() (metric.Emitter, error) {

	client, err := statsd.New(fmt.Sprintf("%s:%s", config.Host, config.Port))
	if err != nil {
		log.Fatal(err)
		return &DogstatsdEmitter{}, err
	}

	if config.Prefix != "" {
		if strings.HasSuffix(config.Prefix, ".") {
			client.Namespace = config.Prefix
		} else {
			client.Namespace = fmt.Sprintf("%s.", config.Prefix)
		}
	}

	return &DogstatsdEmitter{
		client: client,
	}, nil
}

var specialChars = regexp.MustCompile("[^a-zA-Z0-9_]+")

func (emitter *DogstatsdEmitter) Emit(logger lager.Logger, event metric.Event) {
	name := specialChars.ReplaceAllString(strings.Replace(strings.ToLower(event.Name), " ", "_", -1), "")

	tags := []string{
		fmt.Sprintf("host:%s", event.Host),
	}

	for k, v := range event.Attributes {
		tags = append(tags, fmt.Sprintf("%s:%s", k, v))
	}

	err := emitter.client.Gauge(name, event.Value, tags, 1)
	if err != nil {
		logger.Error("failed-to-send-metric",
			errors.Wrap(metric.ErrFailedToEmit, err.Error()))
		return
	}
}
