package baggageclaimcmd

import (
	"fmt"
	"net/http"
	"os"
	"regexp"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/baggageclaim/api"
	"github.com/concourse/concourse/worker/baggageclaim/uidgid"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/flag"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

type BaggageclaimCommand struct {
	Logger flag.Lager

	BindIP   flag.IP `long:"bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for API traffic."`
	BindPort uint16  `long:"bind-port" default:"7788"      description:"Port on which to listen for API traffic."`

	DebugBindIP   flag.IP `long:"debug-bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the pprof debugger endpoints."`
	DebugBindPort uint16  `long:"debug-bind-port" default:"7787"      description:"Port on which to listen for the pprof debugger endpoints."`

	P2pInterfaceNamePattern string `long:"p2p-interface-name-pattern" default:"eth0" description:"Regular expression to match a network interface for p2p streaming"`
	P2pInterfaceFamily int `long:"p2p-interface-family" default:"4" choice:"4" choice:"6" description:"4 for IPv4 and 6 for IPv6"`

	VolumesDir flag.Dir `long:"volumes" required:"true" description:"Directory in which to place volume data."`

	Driver string `long:"driver" default:"detect" choice:"detect" choice:"naive" choice:"btrfs" choice:"overlay" description:"Driver to use for managing volumes."`

	BtrfsBin string `long:"btrfs-bin" default:"btrfs" description:"Path to btrfs binary"`
	MkfsBin  string `long:"mkfs-bin" default:"mkfs.btrfs" description:"Path to mkfs.btrfs binary"`

	OverlaysDir string `long:"overlays-dir" description:"Path to directory in which to store overlay data"`

	DisableUserNamespaces bool `long:"disable-user-namespaces" description:"Disable remapping of user/group IDs in unprivileged volumes."`
}

func (cmd *BaggageclaimCommand) Execute(args []string) error {
	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *BaggageclaimCommand) Runner(args []string) (ifrit.Runner, error) {
	logger, _ := cmd.constructLogger()

	listenAddr := fmt.Sprintf("%s:%d", cmd.BindIP.IP, cmd.BindPort)

	var privilegedNamespacer, unprivilegedNamespacer uidgid.Namespacer

	if !cmd.DisableUserNamespaces && uidgid.Supported() {
		privilegedNamespacer = &uidgid.UidNamespacer{
			Translator: uidgid.NewTranslator(uidgid.NewPrivilegedMapper()),
			Logger:     logger.Session("uid-namespacer"),
		}

		unprivilegedNamespacer = &uidgid.UidNamespacer{
			Translator: uidgid.NewTranslator(uidgid.NewUnprivilegedMapper()),
			Logger:     logger.Session("uid-namespacer"),
		}
	} else {
		privilegedNamespacer = uidgid.NoopNamespacer{}
		unprivilegedNamespacer = uidgid.NoopNamespacer{}
	}

	locker := volume.NewLockManager()

	driver, err := cmd.driver(logger)
	if err != nil {
		logger.Error("failed-to-set-up-driver", err)
		return nil, err
	}

	filesystem, err := volume.NewFilesystem(driver, cmd.VolumesDir.Path())
	if err != nil {
		logger.Error("failed-to-initialize-filesystem", err)
		return nil, err
	}

	err = driver.Recover(filesystem)
	if err != nil {
		logger.Error("failed-to-recover-volume-driver", err)
		return nil, err
	}

	volumeRepo := volume.NewRepository(
		filesystem,
		locker,
		privilegedNamespacer,
		unprivilegedNamespacer,
	)

	re, err := regexp.Compile(cmd.P2pInterfaceNamePattern)
	if err != nil {
		logger.Error("failed-to-compile-p2p-interface-name-pattern", err)
		return nil, err
	}
	apiHandler, err := api.NewHandler(
		logger.Session("api"),
		volume.NewStrategerizer(),
		volumeRepo,
		re,
		cmd.P2pInterfaceFamily,
		cmd.BindPort,
	)
	if err != nil {
		logger.Fatal("failed-to-create-handler", err)
	}

	members := []grouper.Member{
		{Name: "api", Runner: http_server.New(listenAddr, apiHandler)},
		{Name: "debug-server", Runner: http_server.New(
			cmd.debugBindAddr(),
			http.DefaultServeMux,
		)},
	}

	return onReady(grouper.NewParallel(os.Interrupt, members), func() {
		logger.Info("listening", lager.Data{
			"addr": listenAddr,
		})
	}), nil
}

func (cmd *BaggageclaimCommand) constructLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cmd.Logger.Logger("baggageclaim")

	return logger, reconfigurableSink
}

func (cmd *BaggageclaimCommand) debugBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.DebugBindIP, cmd.DebugBindPort)
}

func onReady(runner ifrit.Runner, cb func()) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		process := ifrit.Background(runner)

		subExited := process.Wait()
		subReady := process.Ready()

		for {
			select {
			case <-subReady:
				cb()
				subReady = nil
			case err := <-subExited:
				return err
			case sig := <-signals:
				process.Signal(sig)
			}
		}
	})
}
