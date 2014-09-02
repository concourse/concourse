package testflight_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"github.com/concourse/atc/postgresrunner"
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
)

var _ = Describe("A job with a git resource", func() {
	var postgresRunner postgresrunner.Runner
	var dbProcess ifrit.Process

	var atcConfigFilePath string

	var atcProcess ifrit.Process

	BeforeEach(func() {
		var err error

		postgresRunner = postgresrunner.Runner{
			Port: 5433 + GinkgoParallelNode(),
		}

		dbProcess = ifrit.Envoke(postgresRunner)
		postgresRunner.CreateTestDB()

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
      uri: %[1]s

  - name: some-git-resource-success
    type: git
    source:
      uri: %[1]s
      branch: success

  - name: some-git-resource-no-update
    type: git
    source:
      uri: %[1]s
      branch: no-update

  - name: some-git-resource-failure
    type: git
    source:
      uri: %[1]s
      branch: failure

jobs:
  - name: some-job
    inputs:
      - resource: some-git-resource
    outputs:
      - resource: some-git-resource-success
        params:
          repository: some-git-resource
    config:
      image: %[2]s
      run:
        path: bash
        args: ["-c", "tail -1 some-git-resource/guids | %[3]s"]

  - name: some-failing-job
    inputs:
      - resource: some-git-resource
    outputs:
      - resource: some-git-resource-no-update
        params:
          repository: some-git-resource
      - resource: some-git-resource-failure
        on: [failure]
        params:
          repository: some-git-resource
    config:
      image: %[2]s
      run:
        path: bash
        args: ["-c", "exit 1"]
`, gitserver.URI(), helperRootfs, guidserver.CurlCommand())
		Ω(err).ShouldNot(HaveOccurred())

		err = atcConfigFile.Close()
		Ω(err).ShouldNot(HaveOccurred())

		atcProcess = ifrit.Envoke(&ginkgomon.Runner{
			Name:          "atc",
			AnsiColorCode: "34m",
			Command: exec.Command(
				builtComponents["atc"],
				"-peerAddr", externalAddr+":8081",
				"-config", atcConfigFilePath,
				"-templates", filepath.Join(atcDir, "server", "templates"),
				"-public", filepath.Join(atcDir, "server", "public"),
				"-sqlDataSource", postgresRunner.DataSourceName(),
				"-checkInterval", "5s",
			),
			StartCheck:        "listening",
			StartCheckTimeout: 5 * time.Second,
		})

		Consistently(atcProcess.Wait(), 1*time.Second).ShouldNot(Receive())
	})

	AfterEach(func() {
		atcProcess.Signal(syscall.SIGINT)
		Eventually(atcProcess.Wait(), 10*time.Second).Should(Receive())

		gitserver.Stop(wardenClient)
		guidserver.Stop(wardenClient)

		postgresRunner.DropTestDB()

		dbProcess.Signal(os.Interrupt)
		Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())

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

	It("performs success outputs when the build succeeds, and failure outputs when the build fails", func() {
		masterSHA := gitserver.RevParse("master")
		Ω(masterSHA).ShouldNot(BeEmpty())

		// synchronize on the build triggering
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(HaveLen(1))

		// should have eventually promoted
		Eventually(func() string {
			return gitserver.RevParse("success")
		}, 10*time.Second, 1*time.Second).Should(Equal(masterSHA))

		// should have promoted to failure branch because of on: [falure]
		Eventually(func() string {
			return gitserver.RevParse("failure")
		}, 10*time.Second, 1*time.Second).Should(Equal(masterSHA))

		// should *not* have promoted to no-update branch
		Consistently(func() string {
			return gitserver.RevParse("no-update")
		}, 10*time.Second, 1*time.Second).Should(BeEmpty())
	})
})
