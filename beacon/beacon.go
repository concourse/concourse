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
const reaperForwardAddr = "0.0.0.0:7799"

//go:generate counterfeiter . Closeable
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

//go:generate counterfeiter . BeaconClient
type BeaconClient interface {
	Register(signals <-chan os.Signal, ready chan<- struct{}) error
	RetireWorker(signals <-chan os.Signal, ready chan<- struct{}) error
	LandWorker(signals <-chan os.Signal, ready chan<- struct{}) error
	DeleteWorker(signals <-chan os.Signal, ready chan<- struct{}) error
	DisableKeepAlive()
}

type Beacon struct {
	Logger                  lager.Logger
	Worker                  atc.Worker
	Client                  Client
	GardenForwardAddr       string
	BaggageclaimForwardAddr string
	ReaperForwardAddr       string
	RegistrationMode        RegistrationMode
	KeepAlive               bool
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
			"--baggageclaim "+baggageclaimForwardAddr+" "+
			"--reaper "+reaperForwardAddr,
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

func (beacon *Beacon) DeleteWorker(signals <-chan os.Signal, ready chan<- struct{}) error {
	beacon.Logger.Debug("delete-worker.start")
	return beacon.run("delete-worker", signals, ready)
}

func (beacon *Beacon) DisableKeepAlive() {
	beacon.KeepAlive = false
}

func (beacon *Beacon) run(command string, signals <-chan os.Signal, ready chan<- struct{}) error {
	beacon.Logger.Debug("command-to-run", lager.Data{"cmd": command})

	conn, err := beacon.Client.Dial()
	if err != nil {
		return err
	}
	defer conn.Close()

	var cancelKeepalive chan<- struct{}
	var keepaliveFailed <-chan error

	if beacon.KeepAlive {
		keepaliveFailed, cancelKeepalive = beacon.Client.KeepAlive()
	}

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
	reaperURL, err := url.Parse(beacon.Worker.ReaperAddr)
	if err != nil {
		return fmt.Errorf("failed to parse reaper url: %s", err)
	}

	var gardenForwardAddrRemote = beacon.Worker.GardenAddr
	var reaperForwardAddrRemote = reaperURL.Host
	var bcForwardAddrRemote = bcURL.Host

	if beacon.GardenForwardAddr != "" {
		gardenForwardAddrRemote = beacon.GardenForwardAddr

		if beacon.BaggageclaimForwardAddr != "" {
			bcForwardAddrRemote = beacon.BaggageclaimForwardAddr
		}

		if beacon.ReaperForwardAddr != "" {
			reaperForwardAddrRemote = beacon.ReaperForwardAddr
		}
	}

	beacon.Client.Proxy(gardenForwardAddr, gardenForwardAddrRemote)
	beacon.Logger.Info("forward-garden-config", lager.Data{"forward": gardenForwardAddr, "forward-remote": gardenForwardAddrRemote})
	beacon.Client.Proxy(baggageclaimForwardAddr, bcForwardAddrRemote)
	beacon.Logger.Info("forward-baggageclaim-config", lager.Data{"forward": baggageclaimForwardAddr, "forward-remote": bcForwardAddrRemote})
	beacon.Client.Proxy(reaperForwardAddr, reaperForwardAddrRemote)
	beacon.Logger.Info("forward-reaper-config", lager.Data{"forward": reaperForwardAddr, "forward-remote": reaperForwardAddrRemote})

	close(ready)

	exited := make(chan error, 1)

	go func() {
		exited <- sess.Wait()
	}()

	select {
	case <-signals:
		if beacon.KeepAlive {
			close(cancelKeepalive)
		}
		sess.Close()
		<-exited

		// don't bother waiting for keepalive

		return nil
	case err := <-exited:
		beacon.Logger.Error("failed-to-keep-session-alive", err)
		return err
	case err := <-keepaliveFailed:
		beacon.Logger.Error("failed-to-keep-alive", err)
		return err
	}
}
