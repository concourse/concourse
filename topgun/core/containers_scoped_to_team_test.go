package topgun_test

import (
	"bytes"
	"strconv"

	. "github.com/concourse/concourse/topgun/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

// TODO-Later Move this to a testflight test as there is no relevant IaaS state
var _ = Describe("Container scope", func() {
	Context("when the container is scoped to a team", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml")
		})

		It("is only hijackable by someone in that team", func() {
			By("setting a pipeline for team `main`")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-put-waiting.yml", "-p", "container-scope-test")

			By("triggering the build")
			Fly.Run("unpause-pipeline", "-p", "container-scope-test")
			buildSession := Fly.Start("trigger-job", "-w", "-j", "container-scope-test/simple-job")
			Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

			By("demonstrating we can hijack into all of the containers")
			buildContainers := ContainersBy("build #", "1")
			for i := 1; i <= len(buildContainers); i++ {
				hijackSession := Fly.SpawnInteractive(
					bytes.NewBufferString(strconv.Itoa(i)+"\n"),
					"hijack",
					"-b", "1",
					"hostname",
				)

				<-hijackSession.Exited
				Expect(hijackSession.ExitCode()).To(Equal(0))
			}

			By("creating a separate team")
			setTeamSession := Fly.SpawnInteractive(
				bytes.NewBufferString("y\n"),
				"set-team",
				"--team-name", "no-access",
				"--local-user", "guest",
			)

			<-setTeamSession.Exited
			Expect(setTeamSession.ExitCode()).To(Equal(0))

			By("logging into other team")
			Fly.Run("login", "-n", "no-access", "-u", "guest", "-p", "guest")

			By("not allowing hijacking into any containers")
			failedFly := Fly.Start("hijack", "-b", "1")
			<-failedFly.Exited
			Expect(failedFly.ExitCode()).NotTo(Equal(0))
			Expect(failedFly.Err).To(gbytes.Say("no containers matched your search parameters!"))

			By("logging back into the other team")
			Fly.Run("login", "-n", "main", "-u", AtcUsername, "-p", AtcPassword)

			By("stopping the build")
			Eventually(func() int {
				hijackSession := Fly.Start(
					"hijack",
					"-b", "1",
					"-s", "simple-task",
					"touch", "/tmp/stop-waiting",
				)

				<-hijackSession.Exited
				return hijackSession.ExitCode()

			}).Should(Equal(0))

			Eventually(buildSession).Should(gbytes.Say("done"))
			<-buildSession.Exited
		})
	})
})
