package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/localip"
	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/flag"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

type Certs struct {
	Dir string `long:"certs-dir" description:"Directory to use when creating the resource certificates volume."`
}

type GardenBackend struct {
	UseHoudini bool `long:"use-houdini" description:"Use the insecure Houdini Garden backend."`

	GDN          string    `long:"bin"    default:"gdn" description:"Path to 'gdn' executable (or leave as 'gdn' to find it in $PATH)."`
	GardenConfig flag.File `long:"config"               description:"Path to a config file to use for Garden. You can also specify Garden flags as env vars, e.g. 'CONCOURSE_GARDEN_FOO_BAR=a,b' for '--foo-bar a --foo-bar b'."`

	DNS DNSConfig `group:"DNS Proxy Configuration" namespace:"dns-proxy"`
}

func (cmd WorkerCommand) lessenRequirements(prefix string, command *flags.Command) {
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

	var runner ifrit.Runner
	if cmd.Garden.UseHoudini {
		runner, err = cmd.houdiniRunner(logger)
	} else {
		runner, err = cmd.gdnRunner(logger)
	}
	if err != nil {
		return atc.Worker{}, nil, err
	}

	return worker, runner, nil
}

func (cmd *WorkerCommand) gdnRunner(logger lager.Logger) (ifrit.Runner, error) {
	if binDir := discoverAsset("bin"); binDir != "" {
		// ensure packaged 'gdn' executable is available in $PATH
		err := os.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
		if err != nil {
			return nil, err
		}
	}

	depotDir := filepath.Join(cmd.WorkDir.Path(), "depot")

	// must be readable by other users so unprivileged containers can run their
	// own `initc' process
	err := os.MkdirAll(depotDir, 0755)
	if err != nil {
		return nil, err
	}

	members := grouper.Members{}

	gdnFlags := []string{}

	if cmd.Garden.GardenConfig.Path() != "" {
		gdnFlags = append(gdnFlags, "--config", cmd.Garden.GardenConfig.Path())
	}

	gdnServerFlags := []string{
		"--bind-ip", cmd.BindIP.IP.String(),
		"--bind-port", fmt.Sprintf("%d", cmd.BindPort),

		"--depot", depotDir,
		"--properties-path", filepath.Join(cmd.WorkDir.Path(), "garden-properties.json"),

		"--time-format", "rfc3339",

		// disable graph and grootfs setup; all images passed to Concourse
		// containers are raw://
		"--no-image-plugin",
	}

	gdnServerFlags = append(gdnServerFlags, detectGardenFlags(logger)...)

	if cmd.Garden.DNS.Enable {
		dnsProxyRunner, err := cmd.dnsProxyRunner(logger.Session("dns-proxy"))
		if err != nil {
			return nil, err
		}

		lip, err := localip.LocalIP()
		if err != nil {
			return nil, err
		}

		members = append(members, grouper.Member{
			Name: "dns-proxy",
			Runner: NewLoggingRunner(
				logger.Session("dns-proxy-runner"),
				dnsProxyRunner,
			),
		})

		gdnServerFlags = append(gdnServerFlags, "--dns-server", lip)

		// must permit access to host network in order for DNS proxy address to be
		// reachable
		gdnServerFlags = append(gdnServerFlags, "--allow-host-access")
	}

	gdnArgs := append(gdnFlags, append([]string{"server"}, gdnServerFlags...)...)
	gdnCmd := exec.Command(cmd.Garden.GDN, gdnArgs...)
	gdnCmd.Stdout = os.Stdout
	gdnCmd.Stderr = os.Stderr
	gdnCmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	members = append(members, grouper.Member{
		Name: "gdn",
		Runner: NewLoggingRunner(
			logger.Session("gdn-runner"),
			cmdRunner{gdnCmd},
		),
	})

	return grouper.NewParallel(os.Interrupt, members), nil
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

var gardenEnvPrefix = "CONCOURSE_GARDEN_"

func detectGardenFlags(logger lager.Logger) []string {
	env := os.Environ()

	flags := []string{}
	for _, e := range env {
		spl := strings.SplitN(e, "=", 2)
		if len(spl) != 2 {
			logger.Info("bogus-env", lager.Data{"env": spl})
			continue
		}

		name := spl[0]
		val := spl[1]

		if !strings.HasPrefix(name, gardenEnvPrefix) {
			continue
		}

		strip := strings.Replace(name, gardenEnvPrefix, "", 1)
		flag := flagify(strip)

		logger.Info("forwarding-garden-env-var", lager.Data{
			"env":  name,
			"flag": flag,
		})

		vals := strings.Split(val, ",")

		for _, v := range vals {
			flags = append(flags, "--"+flag, v)
		}

		// clear out env (as twentythousandtonnesofcrudeoil does)
		_ = os.Unsetenv(name)
	}

	return flags
}

func flagify(env string) string {
	return strings.Replace(strings.ToLower(env), "_", "-", -1)
}
