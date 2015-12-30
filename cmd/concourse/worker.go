package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"

	"golang.org/x/crypto/ssh"
)

type WorkerCommand struct {
	WorkDir string `long:"work-dir" required:"true" description:"Directory in which to place container data."`

	BindIP   IPFlag `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for the Garden server."`
	BindPort uint16 `long:"bind-port" default:"7777"    description:"Port on which to listen for the Garden server."`

	PeerIP string `long:"peer-ip" description:"IP used to reach this worker from the ATC nodes. If omitted, the worker will be forwarded through the SSH connection to the TSA."`

	TSA BeaconConfig `group:"TSA Configuration" namespace:"tsa"`
}

type BeaconConfig struct {
	Host             string   `long:"host" default:"127.0.0.1" description:"TSA host to forward the worker through."`
	Port             int      `long:"port" default:"2222" description:"TSA port to connect to."`
	PublicKey        FileFlag `long:"public-key" required:"true" description:"File containing a public key to expect from the TSA."`
	WorkerPrivateKey FileFlag `long:"worker-private-key" required:"true" description:"File containing the private key to use when authenticating to the TSA."`
}

func (config BeaconConfig) Dial() (*ssh.Client, error) {
	workerPrivateKeyBytes, err := ioutil.ReadFile(string(config.WorkerPrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to read worker private key: %s", err)
	}

	workerPrivateKey, err := ssh.ParsePrivateKey(workerPrivateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse worker private key: %s", err)
	}

	clientConfig := &ssh.ClientConfig{
		User: "beacon", // doesn't matter

		HostKeyCallback: config.checkHostKey,

		Auth: []ssh.AuthMethod{ssh.PublicKeys(workerPrivateKey)},
	}

	tsaAddr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	return ssh.Dial("tcp", tsaAddr, clientConfig)
}

func (config BeaconConfig) checkHostKey(hostname string, remote net.Addr, key ssh.PublicKey) error {
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
