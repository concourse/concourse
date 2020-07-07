package workercmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	concourseCmd "github.com/concourse/concourse/cmd"
	"github.com/concourse/flag"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
)

type Certs struct {
	Dir string `long:"certs-dir" description:"Directory to use when creating the resource certificates volume."`
}

type GardenBackend struct {
	UseHoudini    bool `long:"use-houdini"    description:"Use the insecure Houdini Garden backend."`
	UseContainerd bool `long:"use-containerd" description:"Use the containerd backend. (experimental)"`

	Bin            string        `long:"bin"        description:"Path to a garden backend executable (non-absolute names get resolved from $PATH)."`
	Config         flag.File     `long:"config"     description:"Path to a config file to use for the Garden backend. Guardian flags as env vars, e.g. 'CONCOURSE_GARDEN_FOO_BAR=a,b' for '--foo-bar a --foo-bar b'."`
	DNSServers     []string      `long:"dns-server" description:"DNS server IP address to use instead of automatically determined servers. Can be specified multiple times."`
	DNS            DNSConfig     `group:"DNS Proxy Configuration" namespace:"dns-proxy"`
	DenyNetworks   []string      `long:"deny-network" description:"Network ranges to which traffic from containers will be restricted. Can be specified multiple times."`
	RequestTimeout time.Duration `long:"request-timeout" default:"5m" description:"How long to wait for requests to Garden to complete. 0 means no timeout."`
}

func (cmd WorkerCommand) LessenRequirements(prefix string, command *flags.Command) {
	// configured as work-dir/volumes
	command.FindOptionByLongName(prefix + "baggageclaim-volumes").Required = false
}

func (cmd *WorkerCommand) gardenRunner(logger lager.Logger) (atc.Worker, ifrit.Runner, error) {
	err := cmd.checkRoot()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	worker := cmd.Worker.Worker()
	worker.Platform = "linux"

	if cmd.Certs.Dir != "" {
		worker.CertsPath = &cmd.Certs.Dir
	}

	worker.ResourceTypes, err = cmd.loadResources(logger.Session("load-resources"))
	if err != nil {
		return atc.Worker{}, nil, err
	}

	worker.Name, err = cmd.workerName()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	trySetConcourseDirInPATH()

	var runner ifrit.Runner

	switch {
	case cmd.Garden.UseHoudini:
		runner, err = cmd.houdiniRunner(logger)
	case cmd.Garden.UseContainerd:
		runner, err = cmd.containerdRunner(logger)
	default:
		runner, err = cmd.guardianRunner(logger)
	}

	if err != nil {
		return atc.Worker{}, nil, err
	}

	return worker, runner, nil
}

func trySetConcourseDirInPATH() {
	binDir := concourseCmd.DiscoverAsset("bin")
	if binDir == "" {
		return
	}

	err := os.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	if err != nil {
		// programming mistake
		panic(fmt.Errorf("failed to set PATH environment variable: %w", err))
	}
}

var ErrNotRoot = errors.New("worker must be run as root")

func (cmd *WorkerCommand) checkRoot() error {
	currentUser, err := user.Current()
	if err != nil {
		return err
	}

	if currentUser.Uid != "0" {
		return ErrNotRoot
	}

	return nil
}

func (cmd *WorkerCommand) dnsProxyRunner(logger lager.Logger) (ifrit.Runner, error) {
	server, err := cmd.Garden.DNS.Server()
	if err != nil {
		return nil, err
	}

	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		server.NotifyStartedFunc = func() {
			close(ready)
			logger.Info("started")
		}

		serveErr := make(chan error, 1)

		go func() {
			serveErr <- server.ListenAndServe()
		}()

		for {
			select {
			case err := <-serveErr:
				return err
			case <-signals:
				server.Shutdown()
			}
		}
	}), nil
}

func (cmd *WorkerCommand) loadResources(logger lager.Logger) ([]atc.WorkerResourceType, error) {
	var types []atc.WorkerResourceType

	if cmd.ResourceTypes != "" {
		basePath := cmd.ResourceTypes.Path()

		entries, err := ioutil.ReadDir(basePath)
		if err != nil {
			logger.Error("failed-to-read-resources-dir", err)
			return nil, err
		}

		for _, e := range entries {
			meta, err := ioutil.ReadFile(filepath.Join(basePath, e.Name(), "resource_metadata.json"))
			if err != nil {
				logger.Error("failed-to-read-resource-type-metadata", err)
				return nil, err
			}

			var t atc.WorkerResourceType
			err = json.Unmarshal(meta, &t)
			if err != nil {
				logger.Error("failed-to-unmarshal-resource-type-metadata", err)
				return nil, err
			}

			t.Image = filepath.Join(basePath, e.Name(), "rootfs.tgz")

			types = append(types, t)
		}
	}

	return types, nil
}



