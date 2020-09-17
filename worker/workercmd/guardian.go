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

// This prepares the Guardian runtime using the gdn binary.
// The gdn binary exposes Guardian's functionality via a Garden server.
func (cmd *WorkerCommand) guardianRunner(logger lager.Logger) (ifrit.Runner, error) {
	depotDir := filepath.Join(cmd.WorkDir.Path(), "depot")

	// must be readable by other users so unprivileged containers can run their
	// own `initc' process
	err := os.MkdirAll(depotDir, 0755)
	if err != nil {
		return nil, err
	}

	members := grouper.Members{}

	gdnConfigFlag := []string{}

	if cmd.Guardian.Config.Path() != "" {
		gdnConfigFlag = append(gdnConfigFlag, "--config", cmd.Guardian.Config.Path())
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

	gdnServerFlags = append(gdnServerFlags, detectGuardianFlags(logger)...)
	gdnServerFlags = append(gdnServerFlags,
		"--max-containers", strconv.Itoa(cmd.Guardian.MaxContainers),
		"--network-pool", cmd.Guardian.NetworkPool,
	)

	if cmd.Guardian.DNS.Enable {
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

	gdnArgs := append(gdnConfigFlag, append([]string{"server"}, gdnServerFlags...)...)

	bin := "gdn"
	if cmd.Guardian.Bin != "" {
		bin = cmd.Guardian.Bin
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

// This won't detect flags listed in the GuardianRuntime struct
func detectGuardianFlags(logger lager.Logger) []string {
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

		if !strings.HasPrefix(name, guardianEnvPrefix) {
			continue
		}

		strip := strings.Replace(name, guardianEnvPrefix, "", 1)
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
