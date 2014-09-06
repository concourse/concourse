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

	var atcPipelineFilePath string

	var atcProcess ifrit.Process

	var (
		gitServer *gitserver.Server

		successGitServer  *gitserver.Server
		failureGitServer  *gitserver.Server
		noUpdateGitServer *gitserver.Server
	)

	BeforeEach(func() {
		var err error

		postgresRunner = postgresrunner.Runner{
			Port: 5433 + GinkgoParallelNode(),
		}

		dbProcess = ifrit.Envoke(postgresRunner)
		postgresRunner.CreateTestDB()

		guidserver.Start(helperRootfs, wardenClient)

		gitServer = gitserver.Start(helperRootfs, wardenClient)
		gitServer.Commit()

		successGitServer = gitserver.Start(helperRootfs, wardenClient)
		failureGitServer = gitserver.Start(helperRootfs, wardenClient)
		noUpdateGitServer = gitserver.Start(helperRootfs, wardenClient)

		atcPipelineFile, err := ioutil.TempFile("", "atc-pipeline")
		Ω(err).ShouldNot(HaveOccurred())

		atcPipelineFilePath = atcPipelineFile.Name()

		_, err = fmt.Fprintf(
			atcPipelineFile,
			`---
resources:
  - name: some-git-resource
    type: git
    source:
      uri: %[1]s
      branch: master

  - name: some-git-resource-success
    type: git
    source:
      uri: %[2]s
      branch: success

  - name: some-git-resource-failure
    type: git
    source:
      uri: %[3]s
      branch: failure

  - name: some-git-resource-no-update
    type: git
    source:
      uri: %[4]s
      branch: no-update

jobs:
  - name: some-job
    inputs:
      - resource: some-git-resource
    outputs:
      - resource: some-git-resource-success
        params:
          repository: some-git-resource
    config:
      image: %[5]s
      run:
        path: bash
        args: ["-c", "tail -1 some-git-resource/guids | %[6]s"]

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
      image: %[5]s
      run:
        path: bash
        args: ["-c", "exit 1"]
`,
			gitServer.URI(),
			successGitServer.URI(),
			failureGitServer.URI(),
			noUpdateGitServer.URI(),
			helperRootfs,
			guidserver.CurlCommand(),
		)
		Ω(err).ShouldNot(HaveOccurred())

		err = atcPipelineFile.Close()
		Ω(err).ShouldNot(HaveOccurred())

		atcProcess = ifrit.Envoke(&ginkgomon.Runner{
			Name:          "atc",
			AnsiColorCode: "34m",
			Command: exec.Command(
				builtComponents["atc"],
				"-peerAddr", externalAddr+":8081",
				"-pipeline", atcPipelineFilePath,
				"-templates", filepath.Join(atcDir, "web", "templates"),
				"-public", filepath.Join(atcDir, "web", "public"),
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

		gitServer.Stop()
		successGitServer.Stop()
		failureGitServer.Stop()
		noUpdateGitServer.Stop()

		guidserver.Stop(wardenClient)

		postgresRunner.DropTestDB()

		dbProcess.Signal(os.Interrupt)
		Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())

		err := os.Remove(atcPipelineFilePath)
		Ω(err).ShouldNot(HaveOccurred())
	})

	It("builds a repo's initial and later commits", func() {
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(HaveLen(1))
		Ω(guidserver.ReportingGuids()).Should(Equal(gitServer.CommittedGuids()))

		gitServer.Commit()

		Eventually(guidserver.ReportingGuids, 2*time.Minute, 10*time.Second).Should(HaveLen(2))
		Ω(guidserver.ReportingGuids()).Should(Equal(gitServer.CommittedGuids()))
	})

	It("performs success outputs when the build succeeds, and failure outputs when the build fails", func() {
		masterSHA := gitServer.RevParse("master")
		Ω(masterSHA).ShouldNot(BeEmpty())

		// synchronize on the build triggering
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(HaveLen(1))

		// should have eventually promoted
		Eventually(func() string {
			return successGitServer.RevParse("success")
		}, 10*time.Second, 1*time.Second).Should(Equal(masterSHA))

		// should have promoted to failure branch because of on: [failure]
		Eventually(func() string {
			return failureGitServer.RevParse("failure")
		}, 10*time.Second, 1*time.Second).Should(Equal(masterSHA))

		// should *not* have promoted to no-update branch
		Consistently(func() string {
			return noUpdateGitServer.RevParse("no-update")
		}, 10*time.Second, 1*time.Second).Should(BeEmpty())
	})
})
