package beacon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
)

const gardenForwardAddr = "0.0.0.0:7777"
const baggageclaimForwardAddr = "0.0.0.0:7788"

//go:generate counterfeiter . Closable
type Closeable interface {
	Close() error
}

//go:generate counterfeiter . Client

type Client interface {
	KeepAlive() (<-chan error, chan<- struct{})
	NewSession(stdin io.Reader, stdout io.Writer, stderr io.Writer) (Session, error)
	Listen(n, addr string) (net.Listener, error)
	Proxy(from, to string) error
	Dial() (Closeable, error)
}

//go:generate counterfeiter . Session
type Session interface {
	Wait() error
	Close() error
	Start(command string) error
}

type Beacon struct {
	Logger                  lager.Logger
	Worker                  atc.Worker
	Client                  Client
	GardenForwardAddr       string
	BaggageclaimForwardAddr string
	RegistrationMode        RegistrationMode
}

type RegistrationMode string

const (
	Direct  RegistrationMode = "direct"
	Forward RegistrationMode = "forward"
)

func (beacon *Beacon) Register(signals <-chan os.Signal, ready chan<- struct{}) error {
	beacon.Logger.Debug("registering")
	if beacon.RegistrationMode == Direct {
		return beacon.registerDirect(signals, ready)
	}

	return beacon.registerForwarded(signals, ready)
}

func (beacon *Beacon) registerForwarded(signals <-chan os.Signal, ready chan<- struct{}) error {
	beacon.Logger.Debug("forward-worker")
	return beacon.run(
		"forward-worker "+
			"--garden "+gardenForwardAddr+" "+
			"--baggageclaim "+baggageclaimForwardAddr,
		signals,
		ready,
	)
}

func (beacon *Beacon) registerDirect(signals <-chan os.Signal, ready chan<- struct{}) error {
	beacon.Logger.Debug("register-worker")
	return beacon.run("register-worker", signals, ready)
}

func (beacon *Beacon) RetireWorker(signals <-chan os.Signal, ready chan<- struct{}) error {
	beacon.Logger.Debug("retire-worker")
	return beacon.run("retire-worker", signals, ready)
}

func (beacon *Beacon) LandWorker(signals <-chan os.Signal, ready chan<- struct{}) error {
	beacon.Logger.Debug("land-worker")
	return beacon.run("land-worker", signals, ready)
}

func (beacon *Beacon) run(command string, signals <-chan os.Signal, ready chan<- struct{}) error {
	conn, err := beacon.Client.Dial()
	if err != nil {
		return err
	}
	defer conn.Close()

	keepaliveFailed, cancelKeepalive := beacon.Client.KeepAlive()

	workerPayload, err := json.Marshal(beacon.Worker)
	if err != nil {
		return err
	}

	sess, err := beacon.Client.NewSession(
		bytes.NewBuffer(workerPayload),
		os.Stdout,
		os.Stderr,
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %s", err)
	}

	defer sess.Close()
	err = sess.Start(command)
	if err != nil {
		return err
	}

	bcURL, err := url.Parse(beacon.Worker.BaggageclaimURL)
	if err != nil {
		return fmt.Errorf("failed to parse baggageclaim url: %s", err)
	}

	var gardenForwardAddrRemote = beacon.Worker.GardenAddr
	var bcForwardAddrRemote = bcURL.Host

	if beacon.GardenForwardAddr != "" {
		gardenForwardAddrRemote = beacon.GardenForwardAddr

		if beacon.BaggageclaimForwardAddr != "" {
			bcForwardAddrRemote = beacon.BaggageclaimForwardAddr
		}
	}

	beacon.Client.Proxy(gardenForwardAddr, gardenForwardAddrRemote)
	beacon.Client.Proxy(baggageclaimForwardAddr, bcForwardAddrRemote)

	close(ready)

	exited := make(chan error, 1)

	go func() {
		exited <- sess.Wait()
	}()

	select {
	case <-signals:
		close(cancelKeepalive)
		sess.Close()

		<-exited

		// don't bother waiting for keepalive

		return nil
	case err := <-exited:
		return err
	case err := <-keepaliveFailed:
		return err
	}
}
