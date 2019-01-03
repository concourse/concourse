package topgun_test

import (
	"io/ioutil"
	"os"
	"path"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BBR", func() {
	var atcs []boshInstance
	var atc0URL string

	BeforeEach(func() {

		Deploy(
			"deployments/concourse.yml",
			"-o", "operations/bbr.yml",
		)

		waitForRunningWorker()

		atcs = JobInstances("web")
		atc0URL = "http://" + atcs[0].IP + ":8080"

		fly.Login(atcUsername, atcPassword, atc0URL)
	})

	Context("backing up a fresh deployment", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(tmpDir)
		})

		It("backups and restores", func() {
			backupArgs := []string{
				"deployment",
				"-d", deploymentName,
				"backup",
				"--artifact-path", tmpDir,
			}

			Run(nil, "bbr", backupArgs...)

			entries, err := ioutil.ReadDir(tmpDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(entries).To(HaveLen(1))

			restoreArgs := []string{
				"deployment",
				"-d", deploymentName,
				"restore",
				"--artifact-path", path.Join(tmpDir, entries[0].Name()),
			}

			Run(nil, "bbr", restoreArgs...)
		})
	})

	// TODO - fix BBR first
	XContext("restoring a deployment with data to the fresh state", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(tmpDir)
		})

		It("backups and restores", func() {
			backupArgs := []string{
				"deployment",
				"-d", deploymentName,
				"backup",
				"--artifact-path", tmpDir,
			}

			Run(nil, "bbr", backupArgs...)

			entries, err := ioutil.ReadDir(tmpDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(entries).To(HaveLen(1))

			fly.Run("set-pipeline", "-n", "-p", "pipeline", "-c", "./pipelines/get-task.yml")
			pipelines := fly.GetPipelines()
			Expect(pipelines).ToNot(BeEmpty())
			Expect(pipelines[0].Name).To(Equal("pipeline"))

			restoreArgs := []string{
				"deployment",
				"-d", deploymentName,
				"restore",
				"--artifact-path", path.Join(tmpDir, entries[0].Name()),
			}

			Run(nil, "bbr", restoreArgs...)

			pipelines = fly.GetPipelines()
			Expect(pipelines).To(BeEmpty())
		})
	})

})
