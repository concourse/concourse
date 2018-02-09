package beacon

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"golang.org/x/crypto/ssh"
)

func NewSSHClient(logger lager.Logger, config Config) (Client, error) {
	client, err := dial(config)
	return &sshClient{
		logger: logger,
		client: client,
		config: config,
	}, err
}

func dial(config Config) (*ssh.Client, error) {
	workerPrivateKeyBytes, err := ioutil.ReadFile(string(config.WorkerPrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to read worker private key: %s", err)
	}

	workerPrivateKey, err := ssh.ParsePrivateKey(workerPrivateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse worker private key: %s", err)
	}

	tsaAddr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	conn, err := keepaliveDialer("tcp", tsaAddr, 10*time.Second)
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
		return nil, fmt.Errorf("failed to construct client connection: %s", err)
	}

	return ssh.NewClient(clientConn, chans, reqs), nil
}

type sshClient struct {
	logger lager.Logger
	client *ssh.Client
	config Config
	conn   ssh.Conn
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
	return c.Listen(n, addr)
}

func (c *sshClient) Close() error {
	return c.client.Close()
}

func (c *sshClient) NewSession(io.Reader, io.Writer, io.Writer) (Session, error) {

	return c.client.NewSession()
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
