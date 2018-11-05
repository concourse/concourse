package testflight_test

import (
	"encoding/json"
	"time"

	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("fly builds command", func() {
	var (
		testflightHiddenPipeline  = "pipeline1"
		testflightExposedPipeline = "pipeline2"
		mainExposedPipeline       = "pipeline3"
		mainHiddenPipeline        = "pipeline4"
	)

	BeforeEach(func() {
		<-(spawnFlyLogin("-n", "testflight").Exited)

		// hidden pipeline in own team
		pipelineName = testflightHiddenPipeline
		setAndUnpausePipeline("fixtures/hooks.yml")
		fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")

		// exposed pipeline in own team
		pipelineName = testflightExposedPipeline
		By("Setting pipeline" + pipelineName)
		setAndUnpausePipeline("fixtures/hooks.yml")
		fly("expose-pipeline", "-p", pipelineName)
		fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
	})

	BeforeEach(func() {
		wait(spawnFlyLogin("-n", "main"))

		// hidden pipeline in other team
		pipelineName = mainHiddenPipeline
		setAndUnpausePipeline("fixtures/hooks.yml")
		fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")

		// exposed pipeline in other team
		pipelineName = mainExposedPipeline
		setAndUnpausePipeline("fixtures/hooks.yml")
		fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
		fly("expose-pipeline", "-p", pipelineName)
	})

	AfterEach(func() {
		var pipelinesToDestroy = []string{
			testflightHiddenPipeline,
			testflightExposedPipeline,
			mainExposedPipeline,
			mainHiddenPipeline,
		}

		wait(spawnFlyLogin("-t", "main"))

		for _, pipeline := range pipelinesToDestroy {
			fly("destroy-pipeline", "-n", "-p", pipeline)
		}
	})

	Context("when no flags passed", func() {
		Context("being logged into custom team", func() {
			JustBeforeEach(func() {
				<-(spawnFlyLogin("-n", "testflight").Exited)
			})

			It("displays the right info", func() {
				sess := spawnFly("builds")
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
				Expect(sess).To(gbytes.Say(testflightExposedPipeline))
				Expect(sess).To(gbytes.Say(testflightHiddenPipeline))
			})
		})
	})

	Context("when specifying since and until", func() {
		type decodedBuild struct {
			Id        int   `json:"id"`
			StartTime int64 `json:"start_time"`
		}

		var allDecodedBuilds []decodedBuild

		const timeLayout = "2006-01-02 15:04:05"

		BeforeEach(func() {
			wait(spawnFlyLogin("-n", "main"))

			pipelineName = mainExposedPipeline

			fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
			fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
			fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
			fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")

			sess := spawnFly("builds", "-j", inPipeline("some-passing-job"), "--json")
			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			err := json.Unmarshal(sess.Out.Contents(), &allDecodedBuilds)
			Expect(err).ToNot(HaveOccurred())
		})

		It("displays only builds that happened within that range of time", func() {
			sess := spawnFly("builds",
				"--until="+time.Unix(allDecodedBuilds[1].StartTime+1, 0).Local().Format(timeLayout),
				"--since="+time.Unix(allDecodedBuilds[3].StartTime-1, 0).Local().Format(timeLayout),
				"-j", inPipeline("some-passing-job"),
				"--json")
			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			var decodedBuilds []decodedBuild
			err := json.Unmarshal(sess.Out.Contents(), &decodedBuilds)
			Expect(err).ToNot(HaveOccurred())

			Expect(decodedBuilds).To(ConsistOf(allDecodedBuilds[1], allDecodedBuilds[2], allDecodedBuilds[3]))
		})
	})

	Context("when specifying values for team flag", func() {
		BeforeEach(func() {
			wait(spawnFlyLogin("-n", "main"))
		})

		It("retrieves only builds for the teams specified", func() {
			sess := spawnFly("builds", "--team=testflight")
			<-sess.Exited

			Expect(sess.ExitCode()).To(Equal(0))
			Expect(sess).To(gbytes.Say(testflightExposedPipeline))
			Expect(sess).To(gbytes.Say("testflight"), "shows the team name")
			Expect(sess).NotTo(gbytes.Say(mainExposedPipeline))
		})
	})
})
