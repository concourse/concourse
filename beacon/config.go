package beacon

import (
	"bytes"
	"errors"
	"net"

	"github.com/concourse/worker/tsa"
	"golang.org/x/crypto/ssh"
)

type Config struct {
	TSAConfig               tsa.Config       `group:"TSA Configuration" namespace:"tsa"`
	GardenForwardAddr       string           `long:"garden-forward-addr" description:"Garden address to forward through SSH to the TSA."`
	BaggageclaimForwardAddr string           `long:"baggageclaim-forward-addr" description:"Baggageclaim address to forward through SSH to the TSA."`
	RegistrationMode        RegistrationMode `long:"registration-mode" default:"forward" choice:"forward" choice:"direct"`
}

func (config Config) checkHostKey(hostname string, remote net.Addr, remoteKey ssh.PublicKey) error {
	// note: hostname/addr are not verified; they may be behind a load balancer
	// so the definition gets a bit fuzzy

	for _, key := range config.TSAConfig.PublicKey.Keys {
		if key.Type() == remoteKey.Type() && bytes.Equal(key.Marshal(), remoteKey.Marshal()) {
			return nil
		}
	}

	return errors.New("remote host public key mismatch")
}
