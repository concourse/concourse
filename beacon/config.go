package beacon

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"

	"golang.org/x/crypto/ssh"
)

type Config struct {
	Host             string           `long:"host" default:"127.0.0.1" description:"TSA host to forward the worker through."`
	Port             int              `long:"port" default:"2222" description:"TSA port to connect to."`
	PublicKey        FileFlag         `long:"public-key" description:"File containing a public key to expect from the TSA."`
	WorkerPrivateKey FileFlag         `long:"worker-private-key" description:"File containing the private key to use when authenticating to the TSA."`
	RegistrationMode RegistrationMode `long:"registration-mode" default:"forward" choice:"forward" choice:"direct"`
	Retry            bool             `long:"retry" description:"Retry connection on failure"`
}

func (config Config) checkHostKey(hostname string, remote net.Addr, key ssh.PublicKey) error {
	hostPublicKeyBytes, err := ioutil.ReadFile(string(config.PublicKey))
	if err != nil {
		return fmt.Errorf("failed to read host public key: %s", err)
	}

	hostPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(hostPublicKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse host public key: %s", err)
	}

	// note: hostname/addr are not verified; they may be behind a load balancer
	// so the definition gets a bit fuzzy

	if hostPublicKey.Type() != key.Type() || !bytes.Equal(hostPublicKey.Marshal(), key.Marshal()) {
		return errors.New("remote host public key mismatch")
	}

	return nil
}
