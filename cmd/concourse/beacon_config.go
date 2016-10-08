package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

type BeaconConfig struct {
	Host             string   `long:"host" default:"127.0.0.1" description:"TSA host to forward the worker through."`
	Port             int      `long:"port" default:"2222" description:"TSA port to connect to."`
	PublicKey        FileFlag `long:"public-key" description:"File containing a public key to expect from the TSA."`
	WorkerPrivateKey FileFlag `long:"worker-private-key" description:"File containing the private key to use when authenticating to the TSA."`
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

	tsaAddr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	conn, err := net.DialTimeout("tcp", tsaAddr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to TSA: %s", err)
	}

	clientConfig := &ssh.ClientConfig{
		User: "beacon", // doesn't matter

		HostKeyCallback: config.checkHostKey,

		Auth: []ssh.AuthMethod{ssh.PublicKeys(workerPrivateKey)},
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(conn, tsaAddr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to construct client connection:", err)
	}

	return ssh.NewClient(clientConn, chans, reqs), nil
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
