package gardenrunner

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/onsi/ginkgo"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

type Runner struct {
	Command *exec.Cmd

	network string
	addr    string

	bin  string
	argv []string

	binPath    string
	rootFSPath string

	tmpdir    string
	graphPath string

	debugAddr string
}

func New(network, addr string, bin, binPath, rootFSPath, graphPath string, argv ...string) *Runner {
	if graphPath == "" {
		graphPath = os.TempDir()
	}

	return &Runner{
		network: network,
		addr:    addr,

		bin:  bin,
		argv: argv,

		binPath:    binPath,
		rootFSPath: rootFSPath,
		graphPath:  filepath.Join(graphPath, fmt.Sprintf("test-garden-%d", ginkgo.GinkgoParallelNode())),
		tmpdir: filepath.Join(
			os.TempDir(),
			fmt.Sprintf("test-garden-%d", ginkgo.GinkgoParallelNode()),
		),
		debugAddr: fmt.Sprintf("0.0.0.0:%d", 15000+ginkgo.GinkgoParallelNode()),
	}
}

func (r *Runner) DebugAddr() string {
	return r.debugAddr
}

func (r *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := lager.NewLogger("garden-runner")
	logger.RegisterSink(lager.NewWriterSink(ginkgo.GinkgoWriter, lager.DEBUG))

	err := os.MkdirAll(r.tmpdir, 0755)
	if err != nil {
		return err
	}

	depotPath := filepath.Join(r.tmpdir, "containers")
	overlaysPath := filepath.Join(r.tmpdir, "overlays")
	snapshotsPath := filepath.Join(r.tmpdir, "snapshots")

	if err := os.MkdirAll(depotPath, 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(snapshotsPath, 0755); err != nil {
		return err
	}

	gardenArgs := append(
		r.argv,
		"--listenNetwork", r.network,
		"--listenAddr", r.addr,
		"--bin", r.binPath,
		"--rootfs", r.rootFSPath,
		"--depot", depotPath,
		"--debugAddr", r.debugAddr,
		"--overlays", overlaysPath,
		"--snapshots", snapshotsPath,
		"--graph", r.graphPath,
		"--logLevel", "debug",
		"--disableQuotas",
		"--networkPool", fmt.Sprintf("10.250.%d.0/24", ginkgo.GinkgoParallelNode()),
		"--portPoolStart", strconv.Itoa(51000+(1000*ginkgo.GinkgoParallelNode())),
		"--portPoolSize", "1000",
		"--uidPoolStart", strconv.Itoa(10000*ginkgo.GinkgoParallelNode()),
		"--tag", strconv.Itoa(ginkgo.GinkgoParallelNode()),
	)

	var signal os.Signal

	r.Command = exec.Command(r.bin, gardenArgs...)

	process := ifrit.Envoke(&ginkgomon.Runner{
		Name:              "garden-linux",
		Command:           r.Command,
		AnsiColorCode:     "31m",
		StartCheck:        "garden-linux.started",
		StartCheckTimeout: 10 * time.Second,
		Cleanup: func() {
			if signal == syscall.SIGKILL {
				logger.Info("removing-tmp-dirs")
				if err := os.RemoveAll(r.tmpdir); err != nil {
					logger.Error("cleanup-tempdirs-failed", err, lager.Data{"tmpdir": r.tmpdir})
				} else {
					logger.Info("tmp-dirs-removed")
				}
			}
		},
	})

	close(ready)

	var waitErr error

dance:
	for {
		select {
		case signal = <-signals:
			if signal == syscall.SIGKILL {
				logger.Info("received-sigkill")
				if err := r.destroyContainers(); err != nil {
					logger.Error("destroy-containers-failed", err)
					return err
				}
				logger.Info("destroyed-containers")
			}

			process.Signal(syscall.SIGTERM)
		case waitErr = <-process.Wait():
			break dance
		}
	}

	logger.Info("process-exited")

	return waitErr
}

func (r *Runner) TryDial() error {
	conn, dialErr := net.DialTimeout(r.network, r.addr, 100*time.Millisecond)

	if dialErr == nil {
		conn.Close()
		return nil
	}

	return dialErr
}

func (r *Runner) NewClient() client.Client {
	return client.New(connection.New(r.network, r.addr))
}

func (r *Runner) destroyContainers() error {
	client := r.NewClient()

	containers, err := client.Containers(nil)
	if err != nil {
		return err
	}

	for _, container := range containers {
		err := client.Destroy(container.Handle())
		if err != nil {
			return err
		}
	}

	return nil
}
