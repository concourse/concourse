// +build linux

package workercmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/localip"
	concourseCmd "github.com/concourse/concourse/cmd"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

// This runs Guardian using the gdn binary
func (cmd *WorkerCommand) guardianRunner(logger lager.Logger) (ifrit.Runner, error) {
	depotDir := filepath.Join(cmd.WorkDir.Path(), "depot")

	// must be readable by other users so unprivileged containers can run their
	// own `initc' process
	err := os.MkdirAll(depotDir, 0755)
	if err != nil {
		return nil, err
	}

	members := grouper.Members{}

	gdnFlags := []string{}

	if cmd.Garden.Config.Path() != "" {
		gdnFlags = append(gdnFlags, "--config", cmd.Garden.Config.Path())
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

	for _, dnsServer := range cmd.Garden.DNSServers {
		gdnServerFlags = append(gdnServerFlags, "--dns-server", dnsServer)
	}

	for _, denyNetwork := range cmd.Garden.DenyNetworks {
		gdnServerFlags = append(gdnServerFlags, "--deny-network", denyNetwork)
	}

	if cmd.ContainerNetworkPool != "" {
		gdnServerFlags = append(gdnServerFlags, "--network-pool", cmd.ContainerNetworkPool)
	}

	if cmd.Garden.MaxContainers != 0 {
		gdnServerFlags = append(gdnServerFlags, "--max-containers", strconv.Itoa(cmd.Garden.MaxContainers))
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
			Runner: concourseCmd.NewLoggingRunner(
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

	bin := "gdn"
	if cmd.Garden.Bin != "" {
		bin = cmd.Garden.Bin
	}

	gdnCmd := exec.Command(bin, gdnArgs...)
	gdnCmd.Stdout = os.Stdout
	gdnCmd.Stderr = os.Stderr
	gdnCmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	members = append(members, grouper.Member{
		Name: "gdn",
		Runner: concourseCmd.NewLoggingRunner(
			logger.Session("gdn-runner"),
			CmdRunner{gdnCmd},
		),
	})

	return grouper.NewParallel(os.Interrupt, members), nil
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
