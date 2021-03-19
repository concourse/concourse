package workercmd

import (
	"time"

	"github.com/concourse/concourse/atc"
)

type WorkerConfig struct {
	Name     string   `yaml:"name,omitempty"`
	Tags     []string `yaml:"tag,omitempty"`
	TeamName string   `yaml:"team,omitempty"`

	HTTPProxy  string `yaml:"http_proxy,omitempty"`
	HTTPSProxy string `yaml:"https_proxy,omitempty"`
	NoProxy    string `yaml:"no_proxy,omitempty"`

	Ephemeral bool `yaml:"ephemeral,omitempty"`

	Version string `yaml:"version,omitempty"`
}

func (c WorkerConfig) Worker() atc.Worker {
	return atc.Worker{
		Tags:          c.Tags,
		Team:          c.TeamName,
		Name:          c.Name,
		StartTime:     time.Now().Unix(),
		Version:       c.Version,
		HTTPProxyURL:  c.HTTPProxy,
		HTTPSProxyURL: c.HTTPSProxy,
		NoProxy:       c.NoProxy,
		Ephemeral:     c.Ephemeral,
	}
}
