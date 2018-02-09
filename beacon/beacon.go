package beacon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
)

const gardenForwardAddr = "0.0.0.0:7777"
const baggageclaimForwardAddr = "0.0.0.0:7788"

//go:generate counterfeiter . Client

type Client interface {
	KeepAlive() (<-chan error, chan<- struct{})
	NewSession(stdin io.Reader, stdout io.Writer, stderr io.Writer) (Session, error)
	Listen(n, addr string) (net.Listener, error)
	Proxy(from, to string) error
	Close() error
}

//go:generate counterfeiter . Session
type Session interface {
	Wait() error
	Close() error
	Start(command string) error
}

type Beacon struct {
	Logger           lager.Logger
	Worker           atc.Worker
	Client           Client
	RegistrationMode RegistrationMode
}

type RegistrationMode string

const (
	Direct  RegistrationMode = "direct"
	Forward RegistrationMode = "forward"
)

func (beacon *Beacon) Register(signals <-chan os.Signal, ready chan<- struct{}) error {
	if beacon.RegistrationMode == Direct {
		return beacon.registerDirect(signals, ready)
	}

	return beacon.registerForwarded(signals, ready)
}

func (beacon *Beacon) registerForwarded(signals <-chan os.Signal, ready chan<- struct{}) error {
	return beacon.run(
		"forward-worker "+
			"--garden "+gardenForwardAddr+" "+
			"--baggageclaim "+baggageclaimForwardAddr,
		signals,
		ready,
	)
}

func (beacon *Beacon) registerDirect(signals <-chan os.Signal, ready chan<- struct{}) error {
	return beacon.run("register-worker", signals, ready)
}

func (beacon *Beacon) RetireWorker(signals <-chan os.Signal, ready chan<- struct{}) error {
	return beacon.run("retire-worker", signals, ready)
}

func (beacon *Beacon) LandWorker(signals <-chan os.Signal, ready chan<- struct{}) error {
	return beacon.run("land-worker", signals, ready)
}

func (beacon *Beacon) run(command string, signals <-chan os.Signal, ready chan<- struct{}) error {
	defer beacon.Client.Close()

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

	beacon.Client.Proxy(gardenForwardAddr, beacon.Worker.GardenAddr)
	beacon.Client.Proxy(baggageclaimForwardAddr, beacon.Worker.BaggageclaimURL)
	// gardenRemoteListener, err := beacon.Client.Listen("tcp", gardenForwardAddr)
	// if err != nil {
	// 	return fmt.Errorf("failed to listen remotely: %s", err)
	// }

	// go beacon.proxyListenerTo(gardenRemoteListener, beacon.Worker.GardenAddr)

	// bcURL, err := url.Parse(beacon.Worker.BaggageclaimURL)
	// if err != nil {
	// 	return fmt.Errorf("failed to parse baggageclaim url: %s", err)
	// }

	// baggageclaimRemoteListener, err := beacon.Client.Listen("tcp", baggageclaimForwardAddr)
	// if err != nil {
	// 	return fmt.Errorf("failed to listen remotely: %s", err)
	// }

	// go beacon.proxyListenerTo(baggageclaimRemoteListener, bcURL.Host)

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
