package beacon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim/client"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/tsa"
)

const (
	gardenForwardAddr       = "0.0.0.0:7777"
	baggageclaimForwardAddr = "0.0.0.0:7788"
	ReaperPort              = "7799"
	reaperAddr              = "0.0.0.0:" + ReaperPort
)

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

	SweepContainers(garden.Client) error
	ReportContainers(garden.Client) error

	SweepVolumes() error
	ReportVolumes() error

	LandWorker(signals <-chan os.Signal, ready chan<- struct{}) error
	DeleteWorker(signals <-chan os.Signal, ready chan<- struct{}) error
	DisableKeepAlive()
}

type Beacon struct {
	Logger           lager.Logger
	Worker           atc.Worker
	Client           Client
	RegistrationMode RegistrationMode
	KeepAlive        bool

	GardenAddr       string
	GardenClient     garden.Client
	BaggageclaimAddr string

	RebalanceTime time.Duration
}

type RegistrationMode string

const (
	Direct  RegistrationMode = "direct"
	Forward RegistrationMode = "forward"
)

func (beacon *Beacon) Register(signals <-chan os.Signal, ready chan<- struct{}) error {
	beacon.Logger.Debug("registering")
	rebalanceDuration := beacon.RebalanceTime
	bwg := &waitGroupWithCount{
		WaitGroup: new(sync.WaitGroup),
		countMutex: new(sync.Mutex),
	}
	ctx := context.Background()
	cancellableCtx, cancelFunc := context.WithCancel(ctx)

	var rebalanceTicker *time.Ticker

	// When mode is Direct or time is 0, additional connections should not be created.
	if  beacon.RegistrationMode == Direct || beacon.RebalanceTime == 0 {
		rebalanceTicker = time.NewTicker(time.Hour)
		rebalanceTicker.Stop()
	} else {
		rebalanceTicker = time.NewTicker(rebalanceDuration)
	}
	defer rebalanceTicker.Stop()

	registerWorker := func(errChan chan error) {
		defer bwg.Decrement()
		timeOutCtx := context.TODO()
		if beacon.RegistrationMode == Forward && beacon.RebalanceTime != 0 {
			timeOutCtx, _ = context.WithTimeout(cancellableCtx, beacon.RebalanceTime )
		}

		if beacon.RegistrationMode == Direct {
			errChan <- beacon.registerDirect(cancellableCtx, timeOutCtx)
		} else {
			errChan <- beacon.registerForwarded(cancellableCtx, timeOutCtx)
		}
	}

	beacon.Logger.Debug("adding-connection-to-pool")

	latestErrChan := make(chan error, 1)

	bwg.Increment()
	go registerWorker(latestErrChan)

	for {
		select {
		case <-rebalanceTicker.C:
			if beacon.RegistrationMode == Forward && bwg.Count() < 5 {
				bwg.Increment()
				beacon.Logger.Debug("adding-connection-to-pool")
				latestErrChan = make(chan error, 1)
				go registerWorker(latestErrChan)
			}
		case err := <-latestErrChan:
			beacon.Logger.Debug("latest-connection-errored")
			cancelFunc()
			bwg.Wait()
			return err
		case <-signals:
			cancelFunc()
			bwg.Wait()
			return nil
		}
	}


}

func (beacon *Beacon) registerForwarded(ctx context.Context, disableKeepAliveCtx context.Context) error {
	beacon.Logger.Debug("forward-worker")
	return beacon.run(
		"forward-worker "+
			"--garden "+gardenForwardAddr+" "+
			"--baggageclaim "+baggageclaimForwardAddr+" ",
		ctx,
		disableKeepAliveCtx,
	)
}

func (beacon *Beacon) registerDirect(ctx context.Context, disableKeepAliveCtx context.Context) error {
	beacon.Logger.Debug("register-worker")
	return beacon.run("register-worker", ctx, disableKeepAliveCtx)
}

// RetireWorker sends a message via the TSA to retire the worker
func (beacon *Beacon) RetireWorker(signals <-chan os.Signal, ready chan<- struct{}) error {
	beacon.Logger.Debug("retire-worker")

	bwg := &waitGroupWithCount{
		WaitGroup: new(sync.WaitGroup),
		countMutex: new(sync.Mutex),
	}

	ctx := context.Background()
	cancellableCtx, cancelFunc := context.WithCancel(ctx)

	errChan := make(chan error, 1)

	go func(){
		bwg.Increment()
		defer bwg.Decrement()
		errChan <- beacon.run("retire-worker", cancellableCtx, context.TODO())
	}()

	select {
	case err := <-errChan:
		return err
	case <-signals:
		cancelFunc()
		bwg.Wait()
		return nil
	}

}

func (beacon *Beacon) SweepContainers(gardenClient garden.Client) error {
	command := tsa.SweepContainers
	beacon.Logger.Info("sweep", lager.Data{"cmd": command})

	var handleBytes []byte
	var handles []string
	var err error
	err = beacon.executeCommand(func(sess Session) error {
		handleBytes, err = sess.Output(command)
		if err != nil {
			return beacon.logFailure(command, err)
		}

		err = json.Unmarshal(handleBytes, &handles)
		if err != nil {
			beacon.Logger.Error("unmarshall output failed", err)
			return beacon.logFailure(command, err)
		}
		return nil
	})

	if nil != err {
		return err
	}

	beacon.Logger.Debug("received-handles-to-destroy", lager.Data{"num-handles": len(handles)})
	for _, containerHandle := range handles {
		err := gardenClient.Destroy(containerHandle)
		if err != nil {
			_, ok := err.(garden.ContainerNotFoundError)
			if ok {
				continue
			}
			beacon.Logger.Error("failed-to-delete-container", err, lager.Data{"handle": containerHandle})
		}
		beacon.Logger.Debug("destroyed-container", lager.Data{"handle": containerHandle})
	}

	return nil
}

func (beacon *Beacon) SweepVolumes() error {
	command := tsa.SweepVolumes
	beacon.Logger.Info("sweep", lager.Data{"cmd": command})

	var handleBytes []byte
	var handles []string
	var err error
	err = beacon.executeCommand(func(sess Session) error {
		handleBytes, err = sess.Output(command)
		if err != nil {
			return beacon.logFailure(command, err)
		}

		err = json.Unmarshal(handleBytes, &handles)
		if err != nil {
			beacon.Logger.Error("unmarshall-output-failed", err)
			return beacon.logFailure(command, err)
		}
		return nil
	})

	if nil != err {
		return err
	}

	beacon.Logger.Debug("received-handles-to-destroy", lager.Data{"num-handles": len(handles)})
	var beaconBaggageclaimAddress = beacon.BaggageclaimAddr

	if beaconBaggageclaimAddress == "" {
		beaconBaggageclaimAddress = fmt.Sprint("http://", baggageclaimForwardAddr)
	}
	baggageclaimClient := client.NewWithHTTPClient(
		beaconBaggageclaimAddress, &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives:     true,
				ResponseHeaderTimeout: 1 * time.Minute,
			},
		})

	err = baggageclaimClient.DestroyVolumes(beacon.Logger, handles)
	if err != nil {
		beacon.Logger.Error("failed-to-destroy-handles", err)
		return beacon.logFailure(command, err)
	}

	return err
}

func (beacon *Beacon) ReportContainers(gardenClient garden.Client) error {
	command := tsa.ReportContainers
	beacon.Logger.Info("reporting-containers")
	var err error

	containers, err := gardenClient.Containers(garden.Properties{})
	if err != nil {
		return err
	}

	containerHandles := []string{}

	for _, container := range containers {
		containerHandles = append(containerHandles, container.Handle())
	}

	cmdString := command
	for _, handleStr := range containerHandles {
		cmdString = cmdString + " " + handleStr
	}

	err = beacon.executeCommand(func(sess Session) error {
		_, err = sess.Output(cmdString)
		return err
	})
	if err != nil {
		beacon.Logger.Error("failed-to-execute-cmd", err)
		return beacon.logFailure(command, err)
	}

	beacon.Logger.Debug("sucessfully-reported-container-handles", lager.Data{"num-handles": len(containerHandles)})
	return nil
}

func (beacon *Beacon) ReportVolumes() error {
	command := tsa.ReportVolumes

	var beaconBaggageclaimAddress = beacon.BaggageclaimAddr

	if beaconBaggageclaimAddress == "" {
		beaconBaggageclaimAddress = fmt.Sprint("http://", baggageclaimForwardAddr)
	}

	baggageclaimClient := client.NewWithHTTPClient(
		beaconBaggageclaimAddress, &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives:     true,
				ResponseHeaderTimeout: 1 * time.Minute,
			},
		})

	volumes, err := baggageclaimClient.ListVolumes(beacon.Logger, nil)
	if err != nil {
		return beacon.logFailure(command, err)
	}

	cmdString := command
	for _, volume := range volumes {
		cmdString = cmdString + " " + volume.Handle()
	}

	err = beacon.executeCommand(func(sess Session) error {
		_, err = sess.Output(cmdString)
		return err
	})

	if err != nil {
		beacon.Logger.Error("failed-to-execute-cmd", err)
		return beacon.logFailure(command, err)
	}

	beacon.Logger.Debug("sucessfully-reported-volume-handles", lager.Data{"num-handles": len(volumes)})
	return nil
}

func (beacon *Beacon) LandWorker(signals <-chan os.Signal, ready chan<- struct{}) error {
	beacon.Logger.Debug("land-worker")

	bwg := &waitGroupWithCount{
		WaitGroup: new(sync.WaitGroup),
		countMutex: new(sync.Mutex),
	}

	ctx := context.Background()
	cancellableCtx, cancelFunc := context.WithCancel(ctx)

	errChan := make(chan error, 1)

	go func(){
		bwg.Increment()
		defer bwg.Decrement()
		errChan <- beacon.run("land-worker", cancellableCtx, context.TODO())
	}()

	select {
	case err := <-errChan:
		return err
	case <-signals:
		cancelFunc()
		bwg.Wait()
		return nil
	}
}

func (beacon *Beacon) DeleteWorker(signals <-chan os.Signal, ready chan<- struct{}) error {
	beacon.Logger.Debug("delete-worker.start")

	bwg := &waitGroupWithCount{
		WaitGroup: new(sync.WaitGroup),
		countMutex: new(sync.Mutex),
	}

	ctx := context.Background()
	cancellableCtx, cancelFunc := context.WithCancel(ctx)

	errChan := make(chan error, 1)

	go func(){
		bwg.Increment()
		defer bwg.Decrement()
		errChan <- beacon.run("delete-worker", cancellableCtx, context.TODO())
	}()
	select {
	case err := <-errChan:
		return err
	case <-signals:
		cancelFunc()
		bwg.Wait()
		return nil
	}

}

func (beacon *Beacon) DisableKeepAlive() {
	beacon.KeepAlive = false
}

// TODO CC: maybe we should pass `ctx` as the first argument (instead of
// `command` to adhere to go patterns?
func (beacon *Beacon) run(command string, ctx context.Context, disableKeepAliveCtx context.Context) error {
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

	if beacon.GardenAddr != "" {
		gardenForwardAddrRemote = beacon.GardenAddr

		if beacon.BaggageclaimAddr != "" {
			bcForwardAddrRemote = beacon.BaggageclaimAddr
		}
	}

	beacon.Logger.Debug("ssh-forward-config", lager.Data{
		"gardenForwardAddrRemote": gardenForwardAddrRemote,
		"bcForwardAddrRemote":     bcForwardAddrRemote,
	})
	beacon.Client.Proxy(gardenForwardAddr, gardenForwardAddrRemote)
	beacon.Client.Proxy(baggageclaimForwardAddr, bcForwardAddrRemote)

	exited := make(chan error, 1)

	go func() {
		exited <- sess.Wait()
	}()

	if beacon.KeepAlive {
		go func() {
			select {
			case <-ctx.Done():
				close(cancelKeepalive)
			case <-disableKeepAliveCtx.Done():
				close(cancelKeepalive)
			}
		}()
	}

	select {
	case <-ctx.Done():
		sess.Close()
		// TODO Is the blocking line below something than can be removed ? CC & SV
		<-exited
		// don't bother waiting for keepalive
		return nil
	case err := <-exited:
		if err != nil {
			beacon.Logger.Error("failed-waiting-on-remote-command", err)
		}
		return err
	case err := <-keepaliveFailed:
		beacon.Logger.Error("failed-to-keep-alive", err)
		return err
	}
}

func (beacon *Beacon) executeCommand(command func(Session) error) error {
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

	return command(sess)
}

func (beacon *Beacon) logFailure(command string, err error) error {
	beacon.Logger.Error(fmt.Sprintf("failed-to-%s", command), err)
	return err
}


