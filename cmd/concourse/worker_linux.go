package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/guardian/guardiancmd"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/localip"
	"github.com/concourse/atc"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

type Certs struct {
	Dir string `long:"certs-dir" description:"Directory to use when creating the resource certificates volume."`
}

type GardenBackend struct {
	guardiancmd.ServerCommand

	DNS DNSConfig `group:"DNS Proxy Configuration" namespace:"dns-proxy"`
}

func (cmd WorkerCommand) lessenRequirements(command *flags.Command) {
	command.FindOptionByLongName("garden-bind-port").Default = []string{"7777"}

	// configured as work-dir/depot
	command.FindOptionByLongName("garden-depot").Required = false

	// un-configure graph (default /var/gdn/graph)
	command.FindOptionByLongName("garden-graph").Required = false
	command.FindOptionByLongName("garden-graph").Default = []string{}

	// these are provided as assets embedded in the 'concourse' binary
	command.FindOptionByLongName("garden-runtime-plugin").Required = false
	command.FindOptionByLongName("garden-dadoo-bin").Required = false
	command.FindOptionByLongName("garden-init-bin").Required = false
	command.FindOptionByLongName("garden-nstar-bin").Required = false
	command.FindOptionByLongName("garden-tar-bin").Required = false

	// configured as work-dir/volumes
	command.FindOptionByLongName("baggageclaim-volumes").Required = false
}

func (cmd *WorkerCommand) gardenRunner(logger lager.Logger, hasAssets bool) (atc.Worker, ifrit.Runner, error) {
	err := cmd.checkRoot()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	depotDir := filepath.Join(cmd.WorkDir.Path(), "depot")

	// must be readable by other users so unprivileged containers can run their
	// own `initc' process
	err = os.MkdirAll(depotDir, 0755)
	if err != nil {
		return atc.Worker{}, nil, err
	}

	cmd.Garden.Server.BindIP = guardiancmd.IPFlag(cmd.BindIP.IP)
	cmd.Garden.Containers.Dir = depotDir

	cmd.Garden.Network.AllowHostAccess = true

	worker := atc.Worker{
		Platform:  "linux",
		Tags:      cmd.Tags,
		Team:      cmd.TeamName,
		CertsPath: &cmd.Certs.Dir,

		HTTPProxyURL:  cmd.HTTPProxy.String(),
		HTTPSProxyURL: cmd.HTTPSProxy.String(),
		NoProxy:       strings.Join(cmd.NoProxy, ","),
		StartTime:     time.Now().Unix(),
	}

	if hasAssets {
		cmd.Garden.Runtime.Plugin = cmd.assetPath("bin", "runc")
		cmd.Garden.Bin.Dadoo = guardiancmd.FileFlag(cmd.assetPath("bin", "dadoo"))
		cmd.Garden.Bin.Init = guardiancmd.FileFlag(cmd.assetPath("bin", "init"))
		cmd.Garden.Bin.NSTar = guardiancmd.FileFlag(cmd.assetPath("bin", "nstar"))
		cmd.Garden.Bin.Tar = guardiancmd.FileFlag(cmd.assetPath("bin", "tar"))

		iptablesDir := cmd.assetPath("iptables")
		cmd.Garden.Bin.IPTables = guardiancmd.FileFlag(filepath.Join(iptablesDir, "sbin", "iptables"))

		worker.ResourceTypes, err = cmd.extractResources(logger.Session("extract-resources"))
		if err != nil {
			return atc.Worker{}, nil, err
		}
	}

	worker.Name, err = cmd.workerName()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	members := grouper.Members{
		{
			Name:   "garden-runc",
			Runner: &cmd.Garden.ServerCommand,
		},
	}

	if cmd.Garden.DNS.Enable {
		dnsProxyRunner, err := cmd.dnsProxyRunner(logger.Session("dns-proxy"))
		if err != nil {
			return atc.Worker{}, nil, err
		}

		lip, err := localip.LocalIP()
		if err != nil {
			return atc.Worker{}, nil, err
		}

		cmd.Garden.Network.AdditionalDNSServers = append(
			cmd.Garden.Network.AdditionalDNSServers,
			guardiancmd.IPFlag(net.ParseIP(lip)),
		)

		members = append(members, grouper.Member{
			Name:   "dns-proxy",
			Runner: dnsProxyRunner,
		})
	}

	return worker, grouper.NewParallel(os.Interrupt, members), nil
}

func (cmd *WorkerCommand) extractResources(logger lager.Logger) ([]atc.WorkerResourceType, error) {
	var resourceTypes []atc.WorkerResourceType

	resourcesDir := cmd.assetPath("resources")

	infos, err := ioutil.ReadDir(resourcesDir)
	if err != nil {
		logger.Error("failed-to-list-resource-assets", err)
		return nil, err
	}

	for _, info := range infos {
		resourceType := info.Name()

		workerResourceType, err := cmd.extractResource(
			logger.Session("extract", lager.Data{"resource-type": resourceType}),
			resourcesDir,
			resourceType,
		)
		if err != nil {
			logger.Error("failed-to-extract-resource", err)
			return nil, err
		}

		resourceTypes = append(resourceTypes, workerResourceType)
	}

	return resourceTypes, nil
}

func (cmd *WorkerCommand) extractResource(
	logger lager.Logger,
	resourcesDir string,
	resourceType string,
) (atc.WorkerResourceType, error) {
	resourceImagesDir := cmd.assetPath("resource-images")
	tarBin := cmd.assetPath("bin", "tar")

	archive := filepath.Join(resourcesDir, resourceType, "rootfs.tar.gz")

	extractedDir := filepath.Join(resourceImagesDir, resourceType)

	rootfsDir := filepath.Join(extractedDir, "rootfs")
	okMarker := filepath.Join(extractedDir, "ok")

	var version string
	versionFile, err := os.Open(filepath.Join(resourcesDir, resourceType, "version"))
	if err != nil {
		logger.Error("failed-to-read-version", err)
		return atc.WorkerResourceType{}, err
	}

	_, err = fmt.Fscanf(versionFile, "%s", &version)
	if err != nil {
		logger.Error("failed-to-parse-version", err)
		return atc.WorkerResourceType{}, err
	}

	defer versionFile.Close()

	var privileged bool
	_, err = os.Stat(filepath.Join(resourcesDir, resourceType, "privileged"))
	if err == nil {
		privileged = true
	}

	_, err = os.Stat(okMarker)
	if err == nil {
		logger.Info("already-extracted")
	} else {
		logger.Info("extracting")

		err := os.RemoveAll(rootfsDir)
		if err != nil {
			logger.Error("failed-to-clear-out-existing-rootfs", err)
			return atc.WorkerResourceType{}, err
		}

		err = os.MkdirAll(rootfsDir, 0755)
		if err != nil {
			logger.Error("failed-to-create-rootfs-dir", err)
			return atc.WorkerResourceType{}, err
		}

		tar := exec.Command(tarBin, "-zxf", archive, "-C", rootfsDir)

		output, err := tar.CombinedOutput()
		if err != nil {
			logger.Error("failed-to-extract-resource", err, lager.Data{
				"output": string(output),
			})
			return atc.WorkerResourceType{}, err
		}

		ok, err := os.Create(okMarker)
		if err != nil {
			logger.Error("failed-to-create-ok-marker", err)
			return atc.WorkerResourceType{}, err
		}

		err = ok.Close()
		if err != nil {
			logger.Error("failed-to-close-ok-marker", err)
			return atc.WorkerResourceType{}, err
		}
	}

	return atc.WorkerResourceType{
		Type:       resourceType,
		Image:      rootfsDir,
		Version:    version,
		Privileged: privileged,
	}, nil
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
