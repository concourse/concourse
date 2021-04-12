package workercmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	concourseCmd "github.com/concourse/concourse/cmd"
	"github.com/concourse/flag"
	"github.com/tedsuo/ifrit"
)

type Certs struct {
	Dir string `yaml:"certs_dir"`
}

type RuntimeConfiguration struct {
	Runtime string `yaml:"runtime" validate:"runtime"`
}

type GuardianRuntime struct {
	Bin            string        `yaml:"bin"`
	DNS            DNSConfig     `yaml:"dns_proxy"`
	RequestTimeout time.Duration `yaml:"request_timeout"`

	Config      flag.File `yaml:"config"`
	BinaryFlags GdnBinaryFlags
}

type ContainerdRuntime struct {
	Config         flag.File     `yaml:"config"`
	Bin            string        `yaml:"bin"`
	InitBin        string        `yaml:"init_bin"`
	CNIPluginsDir  string        `yaml:"cni_plugins_dir"`
	RequestTimeout time.Duration `yaml:"request_timeout"`

	Network ContainerdNetwork `yaml:"network" ignore_env:"true"`

	MaxContainers int `yaml:"max_containers"`
}

type ContainerdNetwork struct {
	ExternalIP net.IP `yaml:"external_ip"`
	//TODO can DNSConfig be simplifed to just a bool rather than struct with a bool?
	DNS                DNSConfig `yaml:"dns_proxy"`
	DNSServers         []string  `yaml:"dns_server"`
	RestrictedNetworks []string  `yaml:"restricted_network"`
	Pool               string    `yaml:"network_pool"`
	MTU                int       `yaml:"mtu"`
}

var RuntimeDefaults = RuntimeConfiguration{
	Runtime: "guardian",
}

var GuardianDefaults = GuardianRuntime{
	RequestTimeout: 5 * time.Minute,
}

var ContainerdDefaults = ContainerdRuntime{
	InitBin:        "/usr/local/concourse/bin/init",
	CNIPluginsDir:  "/usr/local/concourse/bin",
	RequestTimeout: 5 * time.Minute,
	Network: ContainerdNetwork{
		Pool: "10.80.0.0/16",
	},
	MaxContainers: 250,
}

const (
	containerdRuntime = "containerd"
	guardianRuntime   = "guardian"
	houdiniRuntime    = "houdini"
)

var ValidRuntimes = []string{
	containerdRuntime,
	guardianRuntime,
	houdiniRuntime,
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
		if cmd.hasFlags(guardianEnvPrefix) || cmd.hasFlags(containerdEnvPrefix) {
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
