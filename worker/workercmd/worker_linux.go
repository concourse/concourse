package workercmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
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

type RuntimeConfiguration struct {
	Runtime string `long:"runtime" default:"guardian" choice:"guardian" choice:"containerd" choice:"houdini" description:"Runtime to use with the worker. Please note that Houdini is insecure and doesn't run 'tasks' in containers."`
}

type GuardianRuntime struct {
	Bin            string        `long:"bin"        description:"Path to a garden server executable (non-absolute names get resolved from $PATH)."`
	DNS            DNSConfig     `group:"DNS Proxy Configuration" namespace:"dns-proxy"`
	RequestTimeout time.Duration `long:"request-timeout" default:"5m" description:"How long to wait for requests to the Garden server to complete. 0 means no timeout."`

  Config         flag.File     `long:"config"     description:"Path to a config file to use for the Garden backend. e.g. 'foo-bar=a,b' for '--foo-bar a --foo-bar b'."`
	BinaryFlags GdnBinaryFlags
}

type ContainerdRuntime struct {
	Config         flag.File     `long:"config"     description:"Path to a config file to use for the Containerd daemon."`
	Bin            string        `long:"bin"        description:"Path to a containerd executable (non-absolute names get resolved from $PATH)."`
	RequestTimeout time.Duration `long:"request-timeout" default:"5m" description:"How long to wait for requests to Containerd to complete. 0 means no timeout."`

	//TODO can DNSConfig be simplifed to just a bool rather than struct with a bool?
	DNS                DNSConfig `group:"DNS Proxy Configuration" namespace:"dns-proxy"`
	DNSServers         []string  `long:"dns-server" description:"DNS server IP address to use instead of automatically determined servers. Can be specified multiple times."`
	RestrictedNetworks []string  `long:"restricted-network" description:"Network ranges to which traffic from containers will be restricted. Can be specified multiple times."`
	MaxContainers      int       `long:"max-containers" default:"250" description:"Max container capacity. 0 means no limit."`
	NetworkPool        string    `long:"network-pool" default:"10.80.0.0/16" description:"Network range to use for dynamically allocated container subnets."`
}

const containerdRuntime = "containerd"
const guardianRuntime = "guardian"
const houdiniRuntime = "houdini"

func (cmd WorkerCommand) LessenRequirements(prefix string, command *flags.Command) {
	// configured as work-dir/volumes
	command.FindOptionByLongName(prefix + "baggageclaim-volumes").Required = false
}

// Chooses the appropriate runtime based on CONCOURSE_RUNTIME_TYPE.
// The runtime is represented as a Ifrit runner that must include a Garden Server process. The Garden server exposes API
// endpoints that allow the ATC to make container related requests to the worker.
// The runner may also include additional processes such as the runtime's daemon or a DNS proxy server.
func (cmd *WorkerCommand) gardenServerRunner(logger lager.Logger) (atc.Worker, ifrit.Runner, error) {
	err := cmd.checkRoot()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	err = cmd.verifyRuntimeFlags()
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
	case cmd.Runtime == houdiniRuntime:
		runner, err = cmd.houdiniRunner(logger)
	case cmd.Runtime == containerdRuntime:
		runner, err = cmd.containerdRunner(logger)
	case cmd.Runtime == guardianRuntime:
		runner, err = cmd.guardianRunner(logger)
	default:
		err = fmt.Errorf("unsupported Runtime :%s", cmd.Runtime)
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
	server, err := cmd.Guardian.DNS.Server()
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


func (cmd *WorkerCommand) hasFlags(prefix string) bool {
	env := os.Environ()

	for _, envVar := range env {
		if strings.HasPrefix(envVar, prefix) {
			return true
		}
	}

	return false
}

const guardianEnvPrefix = "CONCOURSE_GARDEN_"
const containerdEnvPrefix = "CONCOURSE_CONTAINERD_"

// Checks if runtime specific flags provided match the selected runtime type
func (cmd *WorkerCommand) verifyRuntimeFlags() error {
	switch {
	case cmd.Runtime == houdiniRuntime:
		if cmd.hasFlags(guardianEnvPrefix)  || cmd.hasFlags(containerdEnvPrefix) {
			return fmt.Errorf("cannot use %s or %s environment variables with Houdini", guardianEnvPrefix, containerdEnvPrefix)
		}
	case cmd.Runtime == containerdRuntime:
		if cmd.hasFlags(guardianEnvPrefix) {
			return fmt.Errorf("cannot use %s environment variables with Containerd", guardianEnvPrefix)
		}
	case cmd.Runtime == guardianRuntime:
		if cmd.hasFlags(containerdEnvPrefix) {
			return fmt.Errorf("cannot use %s environment variables with Guardian", containerdEnvPrefix)
		}
	default:
		return fmt.Errorf("unsupported Runtime :%s", cmd.Runtime)
	}

	return nil
}
