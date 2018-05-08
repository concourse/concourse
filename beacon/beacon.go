package beacon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/worker/reaper"
)

const gardenForwardAddr = "0.0.0.0:7777"
const baggageclaimForwardAddr = "0.0.0.0:7788"
const ReaperPort = "7799"
const reaperAddr = "0.0.0.0:" + ReaperPort

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
	// Read out of session
	Close() error
	Start(command string) error
	Output(command string) ([]byte, error)
}

//go:generate counterfeiter . BeaconClient
type BeaconClient interface {
	Register(signals <-chan os.Signal, ready chan<- struct{}) error
	RetireWorker(signals <-chan os.Signal, ready chan<- struct{}) error
	MarkandSweepContainers() error
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
	ReaperAddr              string
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
			"--baggageclaim "+baggageclaimForwardAddr+" ",
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

func (beacon *Beacon) MarkandSweepContainers() error {
	beacon.Logger.Debug("mark-and-sweep-containers")

	sweepErr := beacon.runSweep("sweep-containers")
	reportErr := beacon.runReport("report-containers")

	var errString string

	if sweepErr != nil {
		errString = fmt.Sprintf("sweep-err:  %s", sweepErr.Error())
	}

	if reportErr != nil {
		errString = errString + fmt.Sprintf("report-err: %s", reportErr.Error())
	}

	if errString == "" {
		return nil
	}
	return errors.New(errString)
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

	var gardenForwardAddrRemote = beacon.Worker.GardenAddr
	var bcForwardAddrRemote = bcURL.Host

	if beacon.GardenForwardAddr != "" {
		gardenForwardAddrRemote = beacon.GardenForwardAddr

		if beacon.BaggageclaimForwardAddr != "" {
			bcForwardAddrRemote = beacon.BaggageclaimForwardAddr
		}
	}

	beacon.Logger.Debug("ssh-forward-config", lager.Data{
		"gardenForwardAddrRemote": gardenForwardAddrRemote,
		"bcForwardAddrRemote":     bcForwardAddrRemote,
	})
	beacon.Client.Proxy(gardenForwardAddr, gardenForwardAddrRemote)
	beacon.Client.Proxy(baggageclaimForwardAddr, bcForwardAddrRemote)

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

func (beacon *Beacon) runReport(command string) error {
	beacon.Logger.Info("command-to-run", lager.Data{"cmd": command})

	conn, err := beacon.Client.Dial()
	if err != nil {
		return err
	}
	defer conn.Close()

	workerPayload, err := json.Marshal(beacon.Worker)
	if err != nil {
		return err
	}

	sess, err := beacon.Client.NewSession(
		bytes.NewBuffer(workerPayload),
		nil,
		os.Stderr,
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %s", err)
	}

	defer sess.Close()

	exited := make(chan error)
	done := make(chan bool)

	go func() {
		var err error

		var reaperAddr = beacon.ReaperAddr

		if reaperAddr == "" {
			reaperAddr = fmt.Sprintf("http://" + reaperAddr)
		}

		rClient := reaper.NewClient(reaperAddr, beacon.Logger.Session("reaper-client"))

		cHandles, err := rClient.ListHandles()
		if err != nil {
			beacon.Logger.Error("failed-to-list-handles", err)
			exited <- err
			return
		}

		cmdString := command
		for _, handleStr := range cHandles {
			cmdString = cmdString + " " + handleStr
		}

		beacon.Logger.Debug("command-running", lager.Data{"cmd": cmdString})

		reportHandleBytes, err := sess.Output(cmdString)
		if err != nil {
			beacon.Logger.Error("failed-to-execute-cmd", err)
			exited <- err
			return
		}
		beacon.Logger.Debug("sucessfully-reported-container-handles", lager.Data{"cmd-output": string(reportHandleBytes)})

		done <- true
	}()

	select {
	case <-done:
		return nil
	case err := <-exited:
		if err != nil {
			beacon.Logger.Error("failed-to-report-containers", err)
			return err
		}
		return nil
	}
}

func (beacon *Beacon) runSweep(command string) error {
	beacon.Logger.Info("command-to-run", lager.Data{"cmd": command})

	conn, err := beacon.Client.Dial()
	if err != nil {
		return err
	}
	defer conn.Close()

	workerPayload, err := json.Marshal(beacon.Worker)
	if err != nil {
		return err
	}

	sess, err := beacon.Client.NewSession(
		bytes.NewBuffer(workerPayload),
		nil,
		os.Stderr,
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %s", err)
	}

	defer sess.Close()

	exited := make(chan error, 1)
	done := make(chan bool)

	go func() {
		var err error
		var handleBytes []byte
		var handles []string

		handleBytes, err = sess.Output(command)
		if err != nil {
			exited <- err
		}

		err = json.Unmarshal(handleBytes, &handles)
		if err != nil {
			beacon.Logger.Error("unmarshall output failed", err)
			exited <- err
			return
		}

		beacon.Logger.Debug("received-handles-to-destroy", lager.Data{"num-handles": len(handles)})

		var reaperAddr = beacon.ReaperAddr

		if reaperAddr == "" {
			reaperAddr = fmt.Sprintf("http://" + reaperAddr)
		}

		rClient := reaper.NewClient(reaperAddr, beacon.Logger.Session("reaper-client"))

		err = rClient.DestroyContainers(handles)
		if err != nil {
			beacon.Logger.Error("failed-to-destroy-handles", err)
			exited <- err
			return
		}
		beacon.Logger.Debug("sucessfully-sweeped-container-handles")

		done <- true
	}()

	select {
	case <-done:
		return nil
	case err := <-exited:
		if err != nil {
			beacon.Logger.Error("failed-to-sweep-containers", err)
			return err
		}
		return nil
	}
}
