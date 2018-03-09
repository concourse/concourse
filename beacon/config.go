package beacon

import (
	"bytes"
	"errors"
	"net"

	"github.com/concourse/flag"
	"golang.org/x/crypto/ssh"
)

type Config struct {
	Host                    string              `long:"host" default:"127.0.0.1" description:"TSA host to forward the worker through."`
	Port                    int                 `long:"port" default:"2222" description:"TSA port to connect to."`
	PublicKey               flag.AuthorizedKeys `long:"public-key" description:"File containing a public key to expect from the TSA."`
	WorkerPrivateKey        flag.PrivateKey     `long:"worker-private-key" description:"File containing the private key to use when authenticating to the TSA."`
	GardenForwardAddr       string              `long:"garden-forward-addr" description:"Garden address to forward through SSH to the TSA."`
	BaggageclaimForwardAddr string              `long:"baggageclaim-forward-addr" description:"Baggageclaim address to forward through SSH to the TSA."`

	Retry bool `long:"retry" description:"Retry connection on failure"`
}

func (config Config) checkHostKey(hostname string, remote net.Addr, remoteKey ssh.PublicKey) error {
	// note: hostname/addr are not verified; they may be behind a load balancer
	// so the definition gets a bit fuzzy

	for _, key := range config.PublicKey.Keys {
		if key.Type() == remoteKey.Type() && bytes.Equal(key.Marshal(), remoteKey.Marshal()) {
			return nil
		}
	}

	return errors.New("remote host public key mismatch")
}
