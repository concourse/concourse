package topgun_test

import (
	"bytes"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Credhub", func() {
	pgDump := func() *gexec.Session {
		dump := exec.Command("pg_dump", "-U", "atc", "-h", dbInstance.IP, "atc")
		dump.Env = append(os.Environ(), "PGPASSWORD=dummy-password")
		dump.Stdin = bytes.NewBufferString("dummy-password\n")
		session, err := gexec.Start(dump, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		<-session.Exited
		Expect(session.ExitCode()).To(Equal(0))
		return session
	}

	getPipeline := func() *gexec.Session {
		session := spawnFly("get-pipeline", "-p", "pipeline-credhub-test")
		<-session.Exited
		Expect(session.ExitCode()).To(Equal(0))
		return session
	}

	credhub := func(args ...string) *gexec.Session {
		login := exec.Command("credhub", "login",
			"-u", os.Getenv("CREDHUB_USERNAME"),
			"-p", os.Getenv("CREDHUB_PASSWORD"),
			"-s", os.Getenv("CREDHUB_URL"),
			"--skip-tls-validation",
		)
		loginSession, err := gexec.Start(login, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		<-loginSession.Exited
		Expect(loginSession.ExitCode()).To(Equal(0))

		cmd := exec.Command("credhub", args...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		<-session.Exited
		Expect(session.ExitCode()).To(Equal(0))
		return session
	}

	BeforeEach(func() {
		if !strings.Contains(string(bosh("releases").Out.Contents()), "credhub") {
			Skip("credhub release not uploaded")
		}
	})

	Describe("A deployment with credhub", func() {
		BeforeEach(func() {
			Deploy(
				"deployments/single-vm.yml",
				"-o", "operations/credhub.yml",
				"-v", "credhub_url="+os.Getenv("CREDHUB_URL"),
				"-v", "credhub_client_id="+os.Getenv("CREDHUB_CLIENT_ID"),
				"-v", "credhub_client_secret="+os.Getenv("CREDHUB_CLIENT_SECRET"),
			)
		})

		Context("with a pipeline build", func() {
			BeforeEach(func() {
				credhub("set", "--type", "value", "--name", "/concourse/main/pipeline-credhub-test/resource_type_repository", "-v", "concourse/time-resource")
				credhub("set", "--type", "value", "--name", "/concourse/main/pipeline-credhub-test/time_resource_interval", "-v", "10m")
				credhub("set", "--type", "user", "--name", "/concourse/main/pipeline-credhub-test/job_secret", "-z", "Hello", "-w", "World")
				credhub("set", "--type", "value", "--name", "/concourse/main/team_secret", "-v", "Sauce")
				credhub("set", "--type", "value", "--name", "/concourse/main/pipeline-credhub-test/image_resource_repository", "-v", "busybox")

				By("setting a pipeline that contains credhub secrets")
				fly("set-pipeline", "-n", "-c", "pipelines/credential-management.yml", "-p", "pipeline-credhub-test")

				By("getting the pipeline config")
				session := getPipeline()
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("concourse/time-resource"))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("10m"))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("Hello/World"))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("Sauce"))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("busybox"))

				By("unpausing the pipeline")
				fly("unpause-pipeline", "-p", "pipeline-credhub-test")
			})

			It("parameterizes via Credhub and leaves the pipeline uninterpolated", func() {
				By("triggering job")
				watch := spawnFly("trigger-job", "-w", "-j", "pipeline-credhub-test/job-with-custom-input")
				wait(watch)
				Expect(watch).To(gbytes.Say("GET SECRET: GET-Hello/GET-World"))
				Expect(watch).To(gbytes.Say("PUT SECRET: PUT-Hello/PUT-World"))
				Expect(watch).To(gbytes.Say("GET SECRET: PUT-GET-Hello/PUT-GET-World"))
				Expect(watch).To(gbytes.Say("SECRET: Hello/World"))
				Expect(watch).To(gbytes.Say("TEAM SECRET: Sauce"))

				By("taking a dump")
				session := pgDump()
				Expect(session).ToNot(gbytes.Say("concourse/time-resource"))
				Expect(session).ToNot(gbytes.Say("10m"))
				Expect(session).To(gbytes.Say("Hello/World")) // build echoed it; nothing we can do
				Expect(session).To(gbytes.Say("Sauce"))       // build echoed it; nothing we can do
				Expect(session).ToNot(gbytes.Say("busybox"))
			})

			Context("when the job's inputs are used for a one-off build", func() {
				It("parameterizes the values using the job's pipeline scope", func() {
					By("triggering job to populate its inputs")
					watch := spawnFly("trigger-job", "-w", "-j", "pipeline-credhub-test/job-with-input")
					wait(watch)
					Expect(watch).To(gbytes.Say("GET SECRET: GET-Hello/GET-World"))
					Expect(watch).To(gbytes.Say("PUT SECRET: PUT-Hello/PUT-World"))
					Expect(watch).To(gbytes.Say("GET SECRET: PUT-GET-Hello/PUT-GET-World"))
					Expect(watch).To(gbytes.Say("SECRET: Hello/World"))
					Expect(watch).To(gbytes.Say("TEAM SECRET: Sauce"))

					By("executing a task that parameterizes image_resource")
					watch = spawnFly("execute", "-c", "tasks/credential-management-with-job-inputs.yml", "-j", "pipeline-credhub-test/job-with-input")
					wait(watch)
					Expect(watch).To(gbytes.Say("./some-resource/input"))

					By("taking a dump")
					session := pgDump()
					Expect(session).ToNot(gbytes.Say("concourse/time-resource"))
					Expect(session).ToNot(gbytes.Say("10m"))
					Expect(session).To(gbytes.Say("./some-resource/input")) // build echoed it; nothing we can do
				})
			})
		})

		Context("with a one-off build", func() {
			BeforeEach(func() {
				credhub("set", "--type", "value", "--name", "/concourse/main/task_secret", "-v", "Hiii")
				credhub("set", "--type", "value", "--name", "/concourse/main/image_resource_repository", "-v", "busybox")
			})

			It("parameterizes image_resource and params in a task config", func() {
				By("executing a task that parameterizes image_resource")
				watch := spawnFly("execute", "-c", "tasks/credential-management.yml")
				wait(watch)
				Expect(watch).To(gbytes.Say("SECRET: Hiii"))

				By("taking a dump")
				session := pgDump()
				Expect(session).ToNot(gbytes.Say("concourse/time-resource"))
				Expect(session).To(gbytes.Say("Hiii")) // build echoed it; nothing we can do
			})
		})
	})
})
