package beacon

import (
	"bytes"
	"errors"
	"net"
	"time"

	"github.com/concourse/concourse/worker/tsa"
	"golang.org/x/crypto/ssh"
)

type Config struct {
	TSAConfig tsa.Config `group:"TSA Configuration" namespace:"tsa"`

	GardenForwardAddr       string           `long:"garden-forward-addr" description:"Garden address to forward through SSH to the TSA."`
	BaggageclaimForwardAddr string           `long:"baggageclaim-forward-addr" description:"Baggageclaim address to forward through SSH to the TSA."`
	Registration           struct {
		Mode                  RegistrationMode `long:"mode" default:"forward" choice:"forward" choice:"direct"`
		RebalanceTime         time.Duration    `long:"rebalance-time" description:"For forwarded mode only. The interval on which a new connection will be created by the worker, also acts as the idle timeout time of the stale connections. A value of 0 would mean that the Worker will not create additional connections." default: "0s"`
	} `group:"Registeration" namespace:"registeration" `
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
