package start

import (
	"time"

	"github.com/concourse/atc"
)

type Config struct {
	Name     string   `long:"name"  description:"The name to set for the worker during registration. If not specified, the hostname will be used."`
	Tags     []string `long:"tag"   description:"A tag to set during registration. Can be specified multiple times."`
	TeamName string   `long:"team"  description:"The name of the team that this worker will be assigned to."`

	HTTPProxy  string `long:"http-proxy"  env:"http_proxy"                  description:"HTTP proxy endpoint to use for containers."`
	HTTPSProxy string `long:"https-proxy" env:"https_proxy"                 description:"HTTPS proxy endpoint to use for containers."`
	NoProxy    string `long:"no-proxy"    env:"no_proxy"                    description:"Blacklist of addresses to skip the proxy when reaching."`

	Ephemeral bool `long:"ephemeral" description:"If set, the worker will be immediately removed upon stalling."`

	DebugBindPort uint16 `long:"bind-debug-port" default:"9099"    description:"Port on which to listen for beacon pprof server."`

	Version string `long:"version" hidden:"true" description:"Version of the worker. This is normally baked in to the binary, so this flag is hidden."`
}

func (c Config) Worker() atc.Worker {
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
