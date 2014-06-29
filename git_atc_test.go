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

	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	"github.com/concourse/testflight/runner"
)

var _ = FDescribe("A job with a git resource", func() {
	var atcConfigFilePath string

	var atcProcess ifrit.Process

	BeforeEach(func() {
		var err error

		guidserver.Start(wardenClient)
		gitserver.Start(wardenClient)

		gitserver.Commit()

		atcConfigFile, err := ioutil.TempFile("", "atc-config")
		Ω(err).ShouldNot(HaveOccurred())

		atcConfigFilePath = atcConfigFile.Name()

		_, err = fmt.Fprintf(atcConfigFile, `---
resources:
	- name: some-git-resource
		type: git
		params:
			uri: %s

jobs:
	- name: some-job
		inputs:
			- resource: some-git-resource
		image: ubuntu
		script: tail -1 some-git-resource/guids | %s
`, gitserver.URI(), guidserver.CurlCommand())
		Ω(err).ShouldNot(HaveOccurred())

		err = atcConfigFile.Close()
		Ω(err).ShouldNot(HaveOccurred())

		atcProcess = ifrit.Envoke(runner.NewRunner(
			builtComponents["atc"],
			"-peerAddr", externalAddr+":8081",
			"-config", atcConfigFilePath,
			"-templates", filepath.Join(atcDir, "server", "templates"),
			"-public", filepath.Join(atcDir, "server", "public"),
		))

		Consistently(atcProcess.Wait(), 1*time.Second).ShouldNot(Receive())
	})

	AfterEach(func() {
		atcProcess.Signal(syscall.SIGINT)
		Eventually(atcProcess.Wait(), 10*time.Second).Should(Receive())

		gitserver.Stop(wardenClient)
		guidserver.Stop(wardenClient)

		err := os.Remove(atcConfigFilePath)
		Ω(err).ShouldNot(HaveOccurred())
	})

	It("builds when the git repo is initialized", func() {
		Eventually(guidserver.ReportingGuids, 2*time.Minute).Should(Equal(gitserver.CommittedGuids()))
	})
})
