package tsa

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
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

// ErrUnauthorized is returned when the client's key is not authorized to act
// on the specified worker.
var ErrUnauthorized = errors.New("key is not authorized to act on the specified worker")

const (
	gardenForwardAddr       = "0.0.0.0:7777"
	baggageclaimForwardAddr = "0.0.0.0:7788"
)

type Client struct {
	Hosts    []string
	HostKeys []ssh.PublicKey

	PrivateKey *rsa.PrivateKey

	Worker atc.Worker
}

type RegisterOptions struct {
	LocalGardenNetwork string
	LocalGardenAddr    string

	LocalBaggageclaimNetwork string
	LocalBaggageclaimAddr    string

	DrainTimeout time.Duration
}

// Register registers a worker with the gateway, continuously keeping the
// connection alive.
//
// If the context times out, the keepalive is stopped, and registration will
// exit after the connection has no activity for the DrainTimeout duration.
//
// If the context is canceled, registration is immediately stopped and the
// connection to the SSH gateway is closed.
func (client *Client) Register(ctx context.Context, opts RegisterOptions) error {
	logger := lagerctx.FromContext(ctx)

	sshClient, tcpConn, err := client.dial(ctx, opts.DrainTimeout)
	if err != nil {
		logger.Error("failed-to-dial", err)
		return err
	}

	defer sshClient.Close()

	go client.keepAlive(ctx, sshClient, tcpConn)

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

	return client.run(
		ctx,
		sshClient,
		"forward-worker --garden "+gardenForwardAddr+" --baggageclaim "+baggageclaimForwardAddr,
		os.Stdout,
	)
}

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
	tsaConn, tsaAddr, err := client.tryDialAll(ctx, idleTimeout)
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
		User: "beacon", // doesn't matter

		HostKeyCallback: client.checkHostKey,

		Auth: []ssh.AuthMethod{ssh.PublicKeys(pk)},
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(tsaConn, tsaAddr, clientConfig)
	if err != nil {
		return nil, nil, ErrUnauthorized
	}

	return ssh.NewClient(clientConn, chans, reqs), tsaConn.(*net.TCPConn), nil
}

func (client *Client) tryDialAll(ctx context.Context, idleTimeout time.Duration) (net.Conn, string, error) {
	logger := lagerctx.FromContext(ctx)

	hosts := map[string]struct{}{}
	for _, host := range client.Hosts {
		hosts[host] = struct{}{}
	}

	for host, _ := range hosts {
		conn, err := keepaliveDialer("tcp", host, 10*time.Second, idleTimeout)
		if err != nil {
			logger.Error("failed-to-connect-to-tsa", err)
			continue
		}

		return conn, host, nil
	}

	return nil, "", ErrAllGatewaysUnreachable
}

func (client *Client) checkHostKey(hostname string, remote net.Addr, remoteKey ssh.PublicKey) error {
	// note: hostname/addr are not verified; they may be behind a load balancer
	// so the definition gets a bit fuzzy

	for _, key := range client.HostKeys {
		if key.Type() == remoteKey.Type() && bytes.Equal(key.Marshal(), remoteKey.Marshal()) {
			return nil
		}
	}

	return errors.New("remote host public key mismatch")
}

func (client *Client) keepAlive(ctx context.Context, sshClient *ssh.Client, tcpConn *net.TCPConn) {
	logger := lagerctx.WithSession(ctx, "keepalive")

	kas := time.NewTicker(5 * time.Second)

	for {
		// ignore reply; server may just not have handled it, since there's no
		// standard keepalive request name

		_, _, err := sshClient.Conn.SendRequest("keepalive", true, []byte("sup"))
		if err != nil {
			logger.Error("failed", err)
			return
		}

		select {
		case <-kas.C:
		case <-ctx.Done():
			if err := tcpConn.SetKeepAlive(false); err != nil {
				logger.Error("failed-to-disable-keepalive", err)
				return
			}

			return
		}
	}
}

func (client *Client) run(ctx context.Context, sshClient *ssh.Client, command string, stdout io.Writer) error {
	logger := lagerctx.WithSession(ctx, "run", lager.Data{
		"command": command,
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
		logger.Info("context-done")
		// XXX: if .Err() is deadline, drain; otherwise, exit
		return nil
	case err := <-errs:
		logger.Error("command-failed", err)
		return err
	}
}

func proxyListenerTo(ctx context.Context, listener net.Listener, network string, addr string) {
	for {
		rConn, err := listener.Accept()
		if err != nil {
			break
		}

		go handleForwardedConn(ctx, rConn, network, addr)
	}
}

func handleForwardedConn(ctx context.Context, rConn net.Conn, network string, addr string) {
	logger := lagerctx.WithSession(ctx, "forward-conn", lager.Data{
		"network": network,
		"addr":    addr,
	})

	defer rConn.Close()

	var lConn net.Conn
	for {
		var err error
		lConn, err = net.Dial("tcp", addr)
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
	go pipe(lConn, rConn)

	wg.Add(1)
	go pipe(rConn, lConn)

	wg.Wait()
}
