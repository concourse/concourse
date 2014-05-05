package redisrunner

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

type Runner struct {
	session *gexec.Session
}

func NewRunner() *Runner {
	return &Runner{}
}

func (runner Runner) Port() int {
	return 6379 + ginkgo.GinkgoParallelNode()
}

func (runner *Runner) Pool() *redis.Pool {
	address := fmt.Sprintf("localhost:%d", runner.Port())

	pool := redis.NewPool(
		func() (redis.Conn, error) {
			return redis.Dial("tcp", address)
		},
		1,
	)

	return pool
}

func (runner *Runner) Start() {
	dir := fmt.Sprintf("/tmp/redis_%d", ginkgo.GinkgoParallelNode())

	err := os.MkdirAll(dir, 0755)
	Ω(err).ShouldNot(HaveOccurred())

	port := fmt.Sprintf("%d", runner.Port())

	redis := exec.Command(
		"redis-server",
		"--port", port,
		"--daemonize", "no",
		"--save", "",
		"--dir", dir,
	)

	runner.session, err = gexec.Start(redis, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())

	Eventually(runner.session).Should(gbytes.Say(
		"The server is now ready to accept connections on port " + port,
	))
}

func (runner *Runner) Stop() {
	runner.session.Interrupt().Wait(5 * time.Second)
}
