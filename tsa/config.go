package tsa

import (
	"github.com/concourse/flag"
)

type Config struct {
	Host             []string            `long:"host" default:"127.0.0.1:2222" description:"TSA host to forward the worker through. Can be specified multiple times."`
	PublicKey        flag.AuthorizedKeys `long:"public-key" description:"File containing a public key to expect from the TSA."`
	WorkerPrivateKey flag.PrivateKey     `long:"worker-private-key" description:"File containing the private key to use when authenticating to the TSA."`
}
