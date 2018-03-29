package beacon

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"golang.org/x/crypto/ssh"
)

var ErrFailedToReachAnyTSA = errors.New("failed to connect to TSA")

func NewSSHClient(logger lager.Logger, config Config) Client {
	return &sshClient{
		logger: logger,
		config: config,
	}
}

type sshClient struct {
	logger lager.Logger
	client *ssh.Client
	config Config
	conn   ssh.Conn
}

func (c *sshClient) Dial() (Closeable, error) {
	tsaAddr := c.config.TSAConfig.Host[rand.Intn(len(c.config.TSAConfig.Host))]

	conn, err := keepaliveDialer("tcp", tsaAddr, 10*time.Second)
	if err != nil {
		c.logger.Error("failed to connect to TSA", err)
		return nil, ErrFailedToReachAnyTSA
	}

	pk, err := ssh.NewSignerFromKey(c.config.TSAConfig.WorkerPrivateKey.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to construct ssh public key from worker key: %s", err)
	}

	clientConfig := &ssh.ClientConfig{
		User: "beacon", // doesn't matter

		HostKeyCallback: c.config.checkHostKey,

		Auth: []ssh.AuthMethod{ssh.PublicKeys(pk)},
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(conn, tsaAddr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to construct client connection: %s", err)
	}

	c.client = ssh.NewClient(clientConn, chans, reqs)

	return c.client, nil
}

func (c *sshClient) KeepAlive() (<-chan error, chan<- struct{}) {
	logger := c.logger.Session("keepalive")

	errs := make(chan error, 1)

	kas := time.NewTicker(5 * time.Second)
	cancel := make(chan struct{})

	go func() {
		for {
			// ignore reply; server may just not have handled it, since there's no
			// standard keepalive request name

			_, _, err := c.client.Conn.SendRequest("keepalive", true, []byte("sup"))
			if err != nil {
				logger.Error("failed", err)
				errs <- err
				return
			}

			logger.Debug("ok")

			select {
			case <-kas.C:
			case <-cancel:
				errs <- nil
				return
			}
		}
	}()

	return errs, cancel
}

func (c *sshClient) Listen(n, addr string) (net.Listener, error) {
	return c.client.Listen(n, addr)
}

func (c *sshClient) Close() error {
	return c.client.Close()
}

func (c *sshClient) NewSession(stdin io.Reader, stdout io.Writer, stderr io.Writer) (Session, error) {
	sess, err := c.client.NewSession()
	if err != nil {
		return nil, err
	}

	sess.Stdin = stdin
	sess.Stdout = stdout
	sess.Stderr = stderr

	return sess, nil
}

func (c *sshClient) Proxy(from, to string) error {
	remoteListener, err := c.Listen("tcp", from)
	if err != nil {
		return fmt.Errorf("failed to listen remotely: %s", err)
	}
	go c.proxyListenerTo(remoteListener, to)
	return nil
}

func (c *sshClient) proxyListenerTo(listener net.Listener, addr string) {
	for {
		rConn, err := listener.Accept()
		if err != nil {
			break
		}

		go c.handleForwardedConn(rConn, addr)
	}
}

func (c *sshClient) handleForwardedConn(rConn net.Conn, addr string) {
	defer rConn.Close()

	lConn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Println("failed to forward remote connection:", err)
		return
	}

	wg := new(sync.WaitGroup)

	pipe := func(to io.WriteCloser, from io.ReadCloser) {
		// if either end breaks, close both ends to ensure they're both unblocked,
		// otherwise io.Copy can block forever if e.g. reading after write end has
		// gone away
		defer to.Close()
		defer from.Close()
		defer wg.Done()

		io.Copy(to, from)
	}

	wg.Add(1)
	go pipe(lConn, rConn)

	wg.Add(1)
	go pipe(rConn, lConn)

	wg.Wait()
}
