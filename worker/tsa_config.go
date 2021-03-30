package worker

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/flag"
	"github.com/concourse/concourse/tsa"
)

type TSAConfig struct {
	Hosts            []string            `yaml:"host"`
	PublicKey        flag.AuthorizedKeys `yaml:"public_key"`
	WorkerPrivateKey *flag.PrivateKey    `yaml:"worker_private_key" validate:"required"`
}

func (config TSAConfig) Client(worker atc.Worker) *tsa.Client {
	return &tsa.Client{
		Hosts:      config.Hosts,
		HostKeys:   config.PublicKey.Keys,
		PrivateKey: config.WorkerPrivateKey.PrivateKey,
		Worker:     worker,
	}
}
