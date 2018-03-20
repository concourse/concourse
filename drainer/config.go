package drainer

import (
	"os"
	"time"

	"github.com/concourse/worker/ssh"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

type Config struct {
	WorkerConfigFile   string         `long:"worker-config-file" description:"Path to worker config file."`
	UserKnownHostsFile string         `long:"user-known-hosts-file" description:"Path to user known hosts file."`
	TSASSHKeyFile      string         `long:"tsa-ssh-key" description:"Path to TSA SSH key."`
	BeaconPidFile      string         `long:"beacon-pid-file" description:"Path to beacon pid file."`
	TSAAddrs           []string       `long:"tsa-addr" description:"Address of a TSA host." value-name:"127.0.0.1:2222"`
	IsShutdown         bool           `long:"shutdown" description:"Whether worker is about to shutdown."`
	Timeout            *time.Duration `long:"timeout" description:"Maximum time to wait for draining to finish."`
}

func (cmd *Config) Execute(args []string) error {
	logger := lager.NewLogger("worker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	logger = logger.Session("drain")

	logger.Debug("running-drain", lager.Data{"shutdown": cmd.IsShutdown})

	sshRunner := ssh.NewRunner(
		ssh.Options{
			Addrs:               cmd.TSAAddrs,
			PrivateKeyFile:      cmd.TSASSHKeyFile,
			UserKnownHostsFile:  cmd.UserKnownHostsFile,
			ConnectTimeout:      30,
			ServerAliveInterval: 8,
			ServerAliveCountMax: 3,
			ConfigFile:          cmd.WorkerConfigFile,
		},
	)

	drainer := &Drainer{
		SSHRunner:    sshRunner,
		IsShutdown:   cmd.IsShutdown,
		WatchProcess: NewBeaconWatchProcess(cmd.BeaconPidFile),
		WaitInterval: 15 * time.Second,
		Clock:        clock.NewClock(),
		Timeout:      cmd.Timeout,
	}

	return drainer.Drain(logger)
}
