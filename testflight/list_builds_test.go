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
		testflightHiddenPipeline  string
		testflightExposedPipeline string
		mainExposedPipeline       string
		mainHiddenPipeline        string
	)

	BeforeEach(func() {
		testflightHiddenPipeline = randomPipelineName()
		testflightExposedPipeline = randomPipelineName()
		mainExposedPipeline = randomPipelineName()
		mainHiddenPipeline = randomPipelineName()

		// hidden pipeline in own team
		fly("set-pipeline", "-n", "-p", testflightHiddenPipeline, "-c", "fixtures/hooks.yml")
		fly("unpause-pipeline", "-p", testflightHiddenPipeline)
		fly("trigger-job", "-j", testflightHiddenPipeline+"/some-passing-job", "-w")

		// exposed pipeline in own team
		fly("set-pipeline", "-n", "-p", testflightExposedPipeline, "-c", "fixtures/hooks.yml")
		fly("unpause-pipeline", "-p", testflightExposedPipeline)
		fly("expose-pipeline", "-p", testflightExposedPipeline)
		fly("trigger-job", "-j", testflightExposedPipeline+"/some-passing-job", "-w")

		withFlyTarget(adminFlyTarget, func() {
			// hidden pipeline in other team
			fly("set-pipeline", "-n", "-p", mainHiddenPipeline, "-c", "fixtures/hooks.yml")
			fly("unpause-pipeline", "-p", mainHiddenPipeline)
			fly("trigger-job", "-j", mainHiddenPipeline+"/some-passing-job", "-w")

			// exposed pipeline in other team
			fly("set-pipeline", "-n", "-p", mainExposedPipeline, "-c", "fixtures/hooks.yml")
			fly("unpause-pipeline", "-p", mainExposedPipeline)
			fly("trigger-job", "-j", mainExposedPipeline+"/some-passing-job", "-w")
			fly("expose-pipeline", "-p", mainExposedPipeline)
		})
	})

	AfterEach(func() {
		fly("destroy-pipeline", "-n", "-p", testflightExposedPipeline)
		fly("destroy-pipeline", "-n", "-p", testflightHiddenPipeline)

		withFlyTarget(adminFlyTarget, func() {
			fly("destroy-pipeline", "-n", "-p", mainExposedPipeline)
			fly("destroy-pipeline", "-n", "-p", mainHiddenPipeline)
		})
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
			withFlyTarget(adminFlyTarget, func() {
				fly("trigger-job", "-j", mainExposedPipeline+"/some-passing-job", "-w")
				fly("trigger-job", "-j", mainExposedPipeline+"/some-passing-job", "-w")
				fly("trigger-job", "-j", mainExposedPipeline+"/some-passing-job", "-w")
				fly("trigger-job", "-j", mainExposedPipeline+"/some-passing-job", "-w")

				sess := spawnFly("builds", "-j", mainExposedPipeline+"/some-passing-job", "--json")
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				err := json.Unmarshal(sess.Out.Contents(), &allDecodedBuilds)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		It("displays only builds that happened within that range of time", func() {
			var decodedBuilds []decodedBuild
			withFlyTarget(adminFlyTarget, func() {
				sess := spawnFly("builds",
					"--until="+time.Unix(allDecodedBuilds[1].StartTime+1, 0).Local().Format(timeLayout),
					"--since="+time.Unix(allDecodedBuilds[3].StartTime-1, 0).Local().Format(timeLayout),
					"-j", mainExposedPipeline+"/some-passing-job",
					"--json")
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				err := json.Unmarshal(sess.Out.Contents(), &decodedBuilds)
				Expect(err).ToNot(HaveOccurred())
			})

			Expect(decodedBuilds).To(ConsistOf(allDecodedBuilds[1], allDecodedBuilds[2], allDecodedBuilds[3]))
		})
	})

	Context("when specifying values for team flag", func() {
		It("retrieves only builds for the teams specified", func() {
			withFlyTarget(adminFlyTarget, func() {
				sess := spawnFly("builds", "--team=testflight")
				<-sess.Exited

				Expect(sess.ExitCode()).To(Equal(0))
				Expect(sess).To(gbytes.Say(testflightExposedPipeline))
				Expect(sess).To(gbytes.Say("testflight"), "shows the team name")
				Expect(sess).NotTo(gbytes.Say(mainExposedPipeline))
			})
		})
	})
})
