package tsa

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"golang.org/x/crypto/ssh"
)

// ErrAllGatewaysUnreachable is returned when all hosts reject the connection.
var ErrAllGatewaysUnreachable = errors.New("all worker SSH gateways unreachable")

// ErrConnectionDrainTimeout is returned when the connection underlying a
// registration has been idle for the configured ConnectionDrainTimeout.
var ErrConnectionDrainTimeout = errors.New("timeout draining connections")

// HandshakeError is returned when the client fails to establish an SSH
// connection, possibly due to bad credentials.
type HandshakeError struct {
	Err error
}

func (err *HandshakeError) Error() string {
	return fmt.Sprintf("failed to establish SSH connection with gateway: %s", err.Err)
}

// These addresses are used to specify which forwarded connection corresponds
// to which component. Note that these aren't actually respected, they just
// have to match between the 'forward-worker' command flags and the SSH reverse
// tunnel configuration.
const (
	gardenForwardAddr       = "0.0.0.0:7777"
	baggageclaimForwardAddr = "0.0.0.0:7788"
)

// Client is used to communicate with a pool of remote SSH gateways.
type Client struct {
	Hosts    []string
	HostKeys []ssh.PublicKey

	PrivateKey *rsa.PrivateKey

	Worker atc.Worker
}

// RegisterOptions contains required configuration for the registration.
type RegisterOptions struct {
	// The local Garden network and address to forward through the SSH gateway.
	LocalGardenNetwork string
	LocalGardenAddr    string

	// The local Baggageclaim network and address to forward through the SSH
	// gateway.
	LocalBaggageclaimNetwork string
	LocalBaggageclaimAddr    string

	// Under normal circumstances, the connection is kept alive by continuously
	// sending a keepalive request to the SSH gateway. When the context is
	// canceled, the keepalive loop is stopped, and the connection will break
	// after it has been idle for this duration, if configured.
	ConnectionDrainTimeout time.Duration

	// RegisteredFunc is called when the initial registration has completed.
	//
	// The function must be careful not to take too long or become deadlocked, or
	// else the SSH connection can starve.
	RegisteredFunc func()

	// HeartbeatedFunc is called on each heartbeat after registration.
	//
	// The function must be careful not to take too long or become deadlocked, or
	// else the SSH connection can starve.
	HeartbeatedFunc func()
}

// Register invokes the 'forward-worker' command, proxying traffic through the
// tunnel and to the configured Garden/Baggageclaim addresses. It will also
// continuously keep the connection alive. The SSH gateway will continuously
// heartbeat the worker.
//
// If the context is canceled, heartbeating is immediately stopped and the
// remote SSH gateway will wait for connections to drain. If a
// ConnectionDrainTimeout is configured, the connection will be terminated
// after no data has gone to/from the SSH gateway for the configured duration.
func (client *Client) Register(ctx context.Context, opts RegisterOptions) error {
	logger := lagerctx.FromContext(ctx)

	sshClient, tcpConn, err := client.dial(ctx, opts.ConnectionDrainTimeout)
	if err != nil {
		logger.Error("failed-to-dial", err)
		return err
	}

	defer sshClient.Close()

	keepAliveInterval := time.Second * 5
	keepAliveTimeout := time.Minute * 5
	go KeepAlive(ctx, sshClient, tcpConn, keepAliveInterval, keepAliveTimeout)

	gardenListener, err := sshClient.Listen("tcp", gardenForwardAddr)
	if err != nil {
		logger.Error("failed-to-listen-for-garden", err)
		return err
	}

	go proxyListenerTo(ctx, gardenListener, opts.LocalGardenNetwork, opts.LocalGardenAddr)

	baggageclaimListener, err := sshClient.Listen("tcp", baggageclaimForwardAddr)
	if err != nil {
		logger.Error("failed-to-listen-for-baggageclaim", err)
		return err
	}

	go proxyListenerTo(ctx, baggageclaimListener, opts.LocalBaggageclaimNetwork, opts.LocalBaggageclaimAddr)

	eventsR, eventsW := io.Pipe()
	defer eventsW.Close()

	events := NewEventReader(eventsR)
	go func() {
		for {
			ev, err := events.Next()
			if err != nil {
				if err != io.EOF {
					logger.Error("failed-to-read-event", err)
				}

				return
			}

			switch ev.Type {
			case EventTypeRegistered:
				if opts.RegisteredFunc != nil {
					opts.RegisteredFunc()
				}

			case EventTypeHeartbeated:
				if opts.HeartbeatedFunc != nil {
					opts.HeartbeatedFunc()
				}
			}
		}
	}()

	err = client.run(
		ctx,
		sshClient,
		"forward-worker --garden "+gardenForwardAddr+" --baggageclaim "+baggageclaimForwardAddr,
		eventsW,
	)
	if err != nil {
		if ctx.Err() != nil && opts.ConnectionDrainTimeout != 0 {
			if _, ok := err.(*ssh.ExitMissingError); ok {
				return ErrConnectionDrainTimeout
			}
		}

		return err
	}

	return nil
}

// Land invokes the 'land-worker' command, which will initiate the landing
// process for the worker. The worker will transition to 'landing' and finally
// to 'landed' when it is fully drained, causing any existing registrations to
// exit.
func (client *Client) Land(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx)

	sshClient, _, err := client.dial(ctx, 0)
	if err != nil {
		logger.Error("failed-to-dial", err)
		return err
	}

	defer sshClient.Close()

	return client.run(ctx, sshClient, "land-worker", os.Stdout)
}

// Retire invokes the 'retire-worker' command, which will initiate the retiring
// process for the worker. The worker will transition to 'retiring' and
// disappear when it is fully drained, causing any existing registrations to
// exit.
func (client *Client) Retire(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx)

	sshClient, _, err := client.dial(ctx, 0)
	if err != nil {
		logger.Error("failed-to-dial", err)
		return err
	}

	defer sshClient.Close()

	return client.run(ctx, sshClient, "retire-worker", os.Stdout)
}

// Delete invokes the 'delete-worker' command, which will immediately
// unregister the worker without draining, causing any existing registrations
// to exit.
func (client *Client) Delete(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx)

	sshClient, _, err := client.dial(ctx, 0)
	if err != nil {
		logger.Error("failed-to-dial", err)
		return err
	}

	defer sshClient.Close()

	return client.run(ctx, sshClient, "delete-worker", os.Stdout)
}

// ContainersToDestroy invokes the 'sweep-containers' command, returning a list
// of handles to be destroyed.
func (client *Client) ContainersToDestroy(ctx context.Context) ([]string, error) {
	logger := lagerctx.FromContext(ctx)

	sshClient, _, err := client.dial(ctx, 0)
	if err != nil {
		logger.Error("failed-to-dial", err)
		return nil, err
	}

	defer sshClient.Close()

	out := new(bytes.Buffer)
	err = client.run(ctx, sshClient, "sweep-containers", out)
	if err != nil {
		return nil, err
	}

	var handles []string
	err = json.Unmarshal(out.Bytes(), &handles)
	if err != nil {
		logger.Error("failed-to-unmarshal-handles", err)
		return nil, err
	}

	return handles, nil
}

// ReportContainers invokes the 'report-containers' command, sending a list of
// the worker's container handles to Concourse.
func (client *Client) ReportContainers(ctx context.Context, handles []string) error {
	logger := lagerctx.FromContext(ctx)

	sshClient, _, err := client.dial(ctx, 0)
	if err != nil {
		logger.Error("failed-to-dial", err)
		return err
	}

	defer sshClient.Close()

	command := append([]string{"report-containers"}, handles...)

	return client.run(ctx, sshClient, strings.Join(command, " "), os.Stdout)
}

// VolumesToDestroy invokes the 'sweep-volumes' command, returning a list of
// handles to be destroyed.
func (client *Client) VolumesToDestroy(ctx context.Context) ([]string, error) {
	logger := lagerctx.FromContext(ctx)

	sshClient, _, err := client.dial(ctx, 0)
	if err != nil {
		logger.Error("failed-to-dial", err)
		return nil, err
	}

	defer sshClient.Close()

	out := new(bytes.Buffer)
	err = client.run(ctx, sshClient, "sweep-volumes", out)
	if err != nil {
		return nil, err
	}

	var handles []string
	err = json.Unmarshal(out.Bytes(), &handles)
	if err != nil {
		logger.Error("failed-to-unmarshal-handles", err)
		return nil, err
	}

	return handles, nil
}

// ReportVolumes invokes the 'report-volumes' command, sending a list of the
// worker's container handles to Concourse.
func (client *Client) ReportVolumes(ctx context.Context, handles []string) error {
	logger := lagerctx.FromContext(ctx)

	sshClient, _, err := client.dial(ctx, 0)
	if err != nil {
		logger.Error("failed-to-dial", err)
		return err
	}

	defer sshClient.Close()

	command := append([]string{"report-volumes"}, handles...)

	return client.run(ctx, sshClient, strings.Join(command, " "), os.Stdout)
}

func (client *Client) dial(ctx context.Context, idleTimeout time.Duration) (*ssh.Client, *net.TCPConn, error) {
	logger := lagerctx.WithSession(ctx, "dial")

	var err error
	tcpConn, tsaAddr, err := client.tryDialAll(ctx)
	if err != nil {
		logger.Error("failed-to-connect-to-any-tsa", err)
		return nil, nil, err
	}

	var pk ssh.Signer
	if client.PrivateKey != nil {
		pk, err = ssh.NewSignerFromKey(client.PrivateKey)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to construct ssh public key from worker key: %s", err)
		}
	} else {
		return nil, nil, fmt.Errorf("private key not provided")
	}

	clientConfig := &ssh.ClientConfig{
		Config: atc.DefaultSSHConfig(),

		User: "beacon", // doesn't matter

		HostKeyCallback: client.checkHostKey,

		Auth: []ssh.AuthMethod{ssh.PublicKeys(pk)},
	}

	tsaConn := tcpConn
	if idleTimeout != 0 {
		tsaConn = &timeoutConn{
			Conn:        tcpConn,
			IdleTimeout: idleTimeout,
		}
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(tsaConn, tsaAddr, clientConfig)
	if err != nil {
		return nil, nil, &HandshakeError{Err: err}
	}

	return ssh.NewClient(clientConn, chans, reqs), tcpConn.(*net.TCPConn), nil
}

func (client *Client) tryDialAll(ctx context.Context) (net.Conn, string, error) {
	logger := lagerctx.FromContext(ctx)

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 15 * time.Second,
	}

	shuffled := make([]string, len(client.Hosts))
	copy(shuffled, client.Hosts)
	shuffle(sort.StringSlice(shuffled))

	for _, host := range shuffled {
		conn, err := dialer.Dial("tcp", host)
		if err != nil {
			logger.Error("failed-to-connect-to-tsa", err)
			continue
		}

		return conn, host, nil
	}

	return nil, "", ErrAllGatewaysUnreachable
}

func (client *Client) checkHostKey(hostname string, remote net.Addr, remoteKey ssh.PublicKey) error {
	// note: hostname/addr are not verified; the TSA may be behind a load
	// balancer so validating it gets a bit more complicated

	for _, key := range client.HostKeys {
		if key.Type() == remoteKey.Type() && bytes.Equal(key.Marshal(), remoteKey.Marshal()) {
			return nil
		}
	}

	return errors.New("remote host public key mismatch")
}


func (client *Client) run(ctx context.Context, sshClient *ssh.Client, command string, stdout io.Writer) error {
	argv := strings.Split(command, " ")
	commandName := ""
	if len(argv) > 0 {
		commandName = argv[0]
	}

	logger := lagerctx.WithSession(ctx, "run", lager.Data{
		"command": commandName,
	})

	sess, err := sshClient.NewSession()
	if err != nil {
		logger.Error("failed-to-open-session", err)
		return err
	}

	defer sess.Close()

	workerPayload, err := json.Marshal(client.Worker)
	if err != nil {
		return err
	}

	sess.Stdin = bytes.NewBuffer(workerPayload)
	sess.Stdout = stdout
	sess.Stderr = os.Stderr

	err = sess.Start(command)
	if err != nil {
		logger.Error("failed-to-start-command", err)
		return err
	}

	errs := make(chan error, 1)
	go func() {
		errs <- sess.Wait()
	}()

	select {
	case <-ctx.Done():
		logger.Info("context-done", lager.Data{
			"context-error": ctx.Err(),
		})

		err := sess.Signal(ssh.SIGINT)
		if err != nil {
			logger.Error("failed-to-send-signal", err)
			return err
		}

		logger.Info("signal-sent")

		err = <-errs
		if err != nil {
			logger.Error("command-exited-after-signal", err)
		} else {
			logger.Debug("command-exited-after-signal")
		}

		return err
	case err := <-errs:
		if err != nil {
			logger.Error("command-failed", err)
			return err
		}

		logger.Debug("command-exited")
		return nil
	}
}

func proxyListenerTo(ctx context.Context, listener net.Listener, network string, addr string) {
	for {
		remoteConn, err := listener.Accept()
		if err != nil {
			break
		}

		go handleForwardedConn(ctx, remoteConn, network, addr)
	}
}

func handleForwardedConn(ctx context.Context, remoteConn net.Conn, network string, addr string) {
	logger := lagerctx.WithSession(ctx, "forward-conn", lager.Data{
		"network": network,
		"addr":    addr,
	})

	defer remoteConn.Close()

	var localConn net.Conn
	for {
		var err error
		localConn, err = net.Dial("tcp", addr)
		if err != nil {
			logger.Error("failed-to-dial", err)
			time.Sleep(time.Second)
			logger.Info("retrying")
			continue
		}

		break
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
	go pipe(localConn, remoteConn)

	wg.Add(1)
	go pipe(remoteConn, localConn)

	wg.Wait()
}

func shuffle(v sort.Interface) {
	for i := v.Len() - 1; i > 0; i-- {
		v.Swap(i, rand.Intn(i+1))
	}
}
