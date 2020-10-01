// +build linux

package workercmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/localip"
	concourseCmd "github.com/concourse/concourse/cmd"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

// Guardian binary flags - these are passed along as-is to the gdn binary as options.
//
// Note: The defaults have been defined to suite Concourse, and are set manually. The go-flags method of setting
// defaults is not used as we need to detect whether the user passed in the value or not.
// This is needed in order to avoid unintentional overrides of user values set in the optional config file.
// See getGdnFlagsFromStruct for details.
type GdnBinaryFlags struct {
	Server struct {
		Network struct {
			Pool string `long:"network-pool" description:"Network range to use for dynamically allocated container subnets. (default:10.80.0.0/16)"`
		} `group:"Container Networking"`

		Limits struct {
			MaxContainers string `long:"max-containers" description:"Maximum container capacity. 0 means no limit. (default:250)"`
		} `group:"Limits"`
	} `group:"server"`
}

// Defaults for GdnBinaryFlags
const (
	defaultGdnMaxContainers = "250"
	defaultGdnNetworkPool   = "10.80.0.0/16"
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

	gdnServerFlags = append(gdnServerFlags, getGdnFlagsFromEnv(logger)...)

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

	var configFlags GdnBinaryFlags
	if cmd.Guardian.Config != "" {
		configFlags, err = getGdnFlagsFromConfig(cmd.Guardian.Config.Path())
		if err != nil {
			return nil, err
		}
	}

	gdnArgs = append(gdnArgs, cmd.getGdnFlagsFromStruct(configFlags)...)

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

// This won't detect flags listed in the GdnBinaryFlags struct because those get unset by the
// twentythousandtonnesofcrudeoil package when passing relevant envs to go-flags
func getGdnFlagsFromEnv(logger lager.Logger) []string {
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

func getGdnFlagsFromConfig(configPath string) (GdnBinaryFlags, error) {
	var configFlags GdnBinaryFlags
	parser := flags.NewParser(&configFlags, flags.Default | flags.IgnoreUnknown)
	parser.NamespaceDelimiter = "-"

	iniParser := flags.NewIniParser(parser)
	err := iniParser.ParseFile(configPath)
	if err != nil {
		return GdnBinaryFlags{}, err
	}

	return configFlags, nil
}

// Following conditions are met in the order given
// 1. Sets GdnBinaryFlag when it has been set by user either via env or CLI option
// 2. Does nothing if flag is present in config.ini
// 3. Sets default value
func (cmd *WorkerCommand) getGdnFlagsFromStruct(configFlags GdnBinaryFlags) []string {
	var cliFlags []string

	if cmd.Guardian.BinaryFlags.Server.Limits.MaxContainers != "" {
		cliFlags = append(cliFlags, "--max-containers", cmd.Guardian.BinaryFlags.Server.Limits.MaxContainers)
	} else if configFlags.Server.Limits.MaxContainers == "" {
		cliFlags = append(cliFlags, "--max-containers", defaultGdnMaxContainers)
	}

	if cmd.Guardian.BinaryFlags.Server.Network.Pool != "" {
		cliFlags = append(cliFlags, "--network-pool", cmd.Guardian.BinaryFlags.Server.Network.Pool)
	} else if configFlags.Server.Network.Pool == "" {
		cliFlags = append(cliFlags, "--network-pool", defaultGdnNetworkPool)
	}

	return cliFlags
}
