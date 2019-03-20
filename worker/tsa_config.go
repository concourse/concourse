package worker

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/tsa"
	"github.com/concourse/flag"
)

type TSAConfig struct {
	Hosts            []string            `long:"host" default:"127.0.0.1:2222" description:"TSA host to forward the worker through. Can be specified multiple times."`
	PublicKey        flag.AuthorizedKeys `long:"public-key" description:"File containing a public key to expect from the TSA."`
	WorkerPrivateKey *flag.PrivateKey    `long:"worker-private-key" required:"true" description:"File containing the private key to use when authenticating to the TSA."`
}

func (config TSAConfig) Client(worker atc.Worker) *tsa.Client {
	return &tsa.Client{
		Hosts:      config.Hosts,
		HostKeys:   config.PublicKey.Keys,
		PrivateKey: config.WorkerPrivateKey.PrivateKey,
		Worker:     worker,
	}
}
