package topgun_test

import (
	"io/ioutil"
	"os"
	"path"
	"strings"

	. "github.com/concourse/concourse/topgun"
	. "github.com/concourse/concourse/topgun/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BBR", func() {

	var (
		atcs       []BoshInstance
		atc0URL    string
		deployArgs []string
	)

	BeforeEach(func() {
		if !strings.Contains(string(Bosh("releases").Out.Contents()), "backup-and-restore-sdk") {
			Skip("backup-and-restore-sdk release not uploaded")
		}
		deployArgs = []string{}
	})

	JustBeforeEach(func() {
		Deploy("deployments/concourse.yml", deployArgs...)

		atcs = JobInstances("web")
		atc0URL = "http://" + atcs[0].IP + ":8080"

		Fly.Login(AtcUsername, AtcPassword, atc0URL)
	})

	Context("using different property providers", func() {

		BeforeEach(func() {
			deployArgs = append(deployArgs, "-v", "worker_instances=0")
		})

		var successfullyExecutesBackup = func() {
			It("successfully executes backup", func() {
				Run(nil, "bbr", "deployment", "-d", DeploymentName, "backup")
			})
		}

		Context("consuming concourse_db links", func() {
			BeforeEach(func() {
				deployArgs = append(deployArgs, "-o", "operations/bbr-concourse-link.yml")
			})

			successfullyExecutesBackup()
		})

		Context("passing properties", func() {
			BeforeEach(func() {
				deployArgs = append(deployArgs, "-o", "operations/bbr-with-properties.yml")
			})

			successfullyExecutesBackup()
		})

	})

	Context("regardless of property provider", func() {

		BeforeEach(func() {
			deployArgs = append(deployArgs, "-o", "operations/bbr-with-properties.yml")
		})

		JustBeforeEach(func() {
			WaitForRunningWorker()
		})

		Context("restoring a deployment with data to a deployment with less data", func() {
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
				By("creating a new pipeline")
				Fly.Run("set-pipeline", "-n", "-p", "pipeline", "-c", "pipelines/get-task.yml")
				pipelines := Fly.GetPipelines()
				Expect(pipelines).ToNot(BeEmpty())
				Expect(pipelines[0].Name).To(Equal("pipeline"))

				By("unpausing the pipeline")
				Fly.Run("unpause-pipeline", "-p", "pipeline")

				By("triggering a build")
				Fly.Run("trigger-job", "-w", "-j", "pipeline/simple-job")

				By("creating a database backup")
				backupArgs := []string{
					"deployment",
					"-d", DeploymentName,
					"backup",
					"--artifact-path", tmpDir,
				}
				Run(nil, "bbr", backupArgs...)
				entries, err := ioutil.ReadDir(tmpDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(entries).To(HaveLen(1))

				By("deleting the deployment")
				WaitForDeploymentAndCompileLocks()
				Bosh("delete-deployment")

				By("creating a new deployment")
				Deploy(
					"deployments/concourse.yml",
					"-o", "operations/bbr-with-properties.yml",
				)
				WaitForRunningWorker()

				atcs = JobInstances("web")
				atc0URL = "http://" + atcs[0].IP + ":8080"

				Fly.Login(AtcUsername, AtcPassword, atc0URL)

				By("restoring the backup")
				restoreArgs := []string{
					"deployment",
					"-d", DeploymentName,
					"restore",
					"--artifact-path", path.Join(tmpDir, entries[0].Name()),
				}
				Run(nil, "bbr", restoreArgs...)
				pipelines = Fly.GetPipelines()
				Expect(pipelines).ToNot(BeEmpty())
				Expect(pipelines[0].Name).To(Equal("pipeline"))
			})
		})

		Context("when restoring fails", func() {
			var tmpDir string

			BeforeEach(func() {
				var err error
				tmpDir, err = ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				os.RemoveAll(tmpDir)
			})

			It("rolls back the partial restore", func() {
				By("creating new pipeline")
				Fly.Run("set-pipeline", "-n", "-p", "pipeline", "-c", "pipelines/get-task.yml")
				pipelines := Fly.GetPipelines()
				Expect(pipelines).ToNot(BeEmpty())
				Expect(pipelines[0].Name).To(Equal("pipeline"))

				By("unpausing the pipeline")
				Fly.Run("unpause-pipeline", "-p", "pipeline")

				By("triggering a build")
				Fly.Run("trigger-job", "-w", "-j", "pipeline/simple-job")

				By("creating a database backup")
				backupArgs := []string{
					"deployment",
					"-d", DeploymentName,
					"backup",
					"--artifact-path", tmpDir,
				}
				Run(nil, "bbr", backupArgs...)
				entries, err := ioutil.ReadDir(tmpDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(entries).To(HaveLen(1))

				By("creating new pipeline and triggering the new pipeling (this will fail the restore)")

				Fly.Run("set-pipeline", "-n", "-p", "pipeline-2", "-c", "pipelines/get-task.yml")
				pipelines = Fly.GetPipelines()
				Expect(pipelines).ToNot(BeEmpty())
				Expect(pipelines[1].Name).To(Equal("pipeline-2"))

				By("unpausing the pipeline")
				Fly.Run("unpause-pipeline", "-p", "pipeline-2")

				By("triggering a build")
				Fly.Run("trigger-job", "-w", "-j", "pipeline-2/simple-job")

				By("restoring concourse")

				restoreArgs := []string{
					"deployment",
					"-d", DeploymentName,
					"restore",
					"--artifact-path", path.Join(tmpDir, entries[0].Name()),
				}
				session := Start(nil, "bbr", restoreArgs...)
				<-session.Exited
				Expect(session.ExitCode()).To(Equal(1))

				By("checking pipeline")
				pipelines = Fly.GetPipelines()
				Expect(pipelines).ToNot(BeEmpty())
				Expect(len(pipelines)).To(Equal(2))
			})
		})

	})
})
