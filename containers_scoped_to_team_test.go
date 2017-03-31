package topgun_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bytes"

	"strconv"

	"github.com/onsi/gomega/gbytes"
)

var _ = Describe(":life [#136140165] Container scope", func() {
	Context("when the container is scoped to a team", func() {
		BeforeEach(func() {
			Deploy("deployments/single-vm.yml")
		})

		It("is only hijackable by someone in that team", func() {
			By("setting a pipeline for team `main`")
			fly("set-pipeline", "-n", "-c", "pipelines/get-task-put-waiting.yml", "-p", "container-scope-test")

			By("triggering the build")
			fly("unpause-pipeline", "-p", "container-scope-test")
			buildSession := spawnFly("trigger-job", "-w", "-j", "container-scope-test/simple-job")
			Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

			By("demonstrating we can hijack into all of the containers")
			buildContainers := containersBy("build #", "1")
			for i := 1; i <= len(buildContainers); i++ {
				hijackSession := spawnFlyInteractive(
					bytes.NewBufferString(strconv.Itoa(i)+"\n"),
					"hijack",
					"-b", "1",
					"hostname",
				)

				<-hijackSession.Exited
				Expect(hijackSession.ExitCode()).To(Equal(0))
			}

			By("creating a separate team")
			setTeamSession := spawnFlyInteractive(
				bytes.NewBufferString("y\n"),
				"set-team",
				"--team-name", "no-access",
				"--no-really-i-dont-want-any-auth",
			)

			<-setTeamSession.Exited
			Expect(setTeamSession.ExitCode()).To(Equal(0))

			By("logging into other team")
			fly("login", "-n", "no-access")

			By("not allowing hijacking into any containers")
			failedFly := spawnFly("hijack", "-b", "1")
			<-failedFly.Exited
			Expect(failedFly.ExitCode()).NotTo(Equal(0))
			Expect(failedFly.Err).To(gbytes.Say("no containers matched your search parameters!"))

			By("logging back into the other team")
			fly("login", "-n", "main")

			By("stopping the build")
			hijackSession := spawnFly(
				"hijack",
				"-b", "1",
				"-s", "simple-task",
				"touch", "/tmp/stop-waiting",
			)

			<-hijackSession.Exited
			Expect(hijackSession.ExitCode()).To(Equal(0))

			Eventually(buildSession).Should(gbytes.Say("done"))
			<-buildSession.Exited
		})
	})
})
