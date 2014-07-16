package testflight_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	"github.com/concourse/atc/redisrunner"
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	"github.com/concourse/testflight/runner"
)

var _ = Describe("A job with a git resource", func() {
	var redisRunner *redisrunner.Runner

	var atcConfigFilePath string

	var atcProcess ifrit.Process

	BeforeEach(func() {
		var err error

		redisRunner = redisrunner.NewRunner()
		redisRunner.Start()

		guidserver.Start(helperRootfs, wardenClient)
		gitserver.Start(helperRootfs, wardenClient)

		gitserver.Commit()

		atcConfigFile, err := ioutil.TempFile("", "atc-config")
		Ω(err).ShouldNot(HaveOccurred())

		atcConfigFilePath = atcConfigFile.Name()

		_, err = fmt.Fprintf(atcConfigFile, `---
resources:
  - name: some-git-resource
    type: git
    source:
      uri: %s

jobs:
  - name: some-job
    inputs:
      - resource: some-git-resource
    config:
      image: %s
      run:
        path: bash
        args:
          - -c
          - tail -1 some-git-resource/guids | %s
`, gitserver.URI(), helperRootfs, guidserver.CurlCommand())
		Ω(err).ShouldNot(HaveOccurred())

		err = atcConfigFile.Close()
		Ω(err).ShouldNot(HaveOccurred())

		atcProcess = ifrit.Envoke(runner.NewRunner(
			builtComponents["atc"],
			"-peerAddr", externalAddr+":8081",
			"-config", atcConfigFilePath,
			"-templates", filepath.Join(atcDir, "server", "templates"),
			"-public", filepath.Join(atcDir, "server", "public"),
			"-redisAddr", fmt.Sprintf("127.0.0.1:%d", redisRunner.Port()),
			"-checkInterval", "10s",
		))

		Consistently(atcProcess.Wait(), 1*time.Second).ShouldNot(Receive())
	})

	AfterEach(func() {
		atcProcess.Signal(syscall.SIGINT)
		Eventually(atcProcess.Wait(), 10*time.Second).Should(Receive())

		gitserver.Stop(wardenClient)
		guidserver.Stop(wardenClient)

		redisRunner.Stop()

		err := os.Remove(atcConfigFilePath)
		Ω(err).ShouldNot(HaveOccurred())
	})

	It("builds a repo's initial and later commits", func() {
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(HaveLen(1))
		Ω(guidserver.ReportingGuids()).Should(Equal(gitserver.CommittedGuids()))

		gitserver.Commit()

		Eventually(guidserver.ReportingGuids, 2*time.Minute, 10*time.Second).Should(HaveLen(2))
		Ω(guidserver.ReportingGuids()).Should(Equal(gitserver.CommittedGuids()))
	})
})
