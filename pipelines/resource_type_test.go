package pipelines_test

import (
	"fmt"

	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Configuring a resource type in a pipeline config", func() {
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)
		originGitServer.CommitResource()
		originGitServer.CommitFileToBranch("initial", "initial", "trigger")
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	Context("with custom resource types", func() {
		BeforeEach(func() {
			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-types.yml",
				"-v", "origin-git-server="+originGitServer.URI(),
				"-y", "privileged=true",
			)
		})

		It("can use custom resource types for 'get', 'put', and task 'image_resource's", func() {
			watch := flyHelper.Watch(pipelineName, "resource-getter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))

			watch = flyHelper.Watch(pipelineName, "resource-putter")
			Expect(watch).To(gbytes.Say("pushing using custom resource"))
			Expect(watch).To(gbytes.Say("some-output/some-file"))

			watch = flyHelper.Watch(pipelineName, "resource-imgur")
			Expect(watch).To(gbytes.Say("fetched from custom resource"))
			Expect(watch).To(gbytes.Say("SOME_ENV=yep"))
		})

		It("should be able to run privileged operations in 'check', 'get' and 'put' steps", func() {
			watch := flyHelper.TriggerJob(pipelineName, "failing-task")
			<-watch.Exited
			// time.Sleep(time.Hour)
			By("hijacking into get container for my-resource")
			hijackS, stdin := flyHelper.HijackInteractive(
				"-j", pipelineName+"/failing-task",
				"-s", "my-resource",
				"--", "sh", "-c",
				"mkdir tmp && mount -o size=50m -t tmpfs swap tmp && exit $?")
			Eventually(hijackS).Should(gbytes.Say("[1-9]*: build #1, step: my-resource, type: get"))
			fmt.Fprintln(stdin, "1")
			Eventually(hijackS).Should(gexec.Exit(0))

			By("hijacking into put container for my-resource")
			hijackS, stdin = flyHelper.HijackInteractive(
				"-j", pipelineName+"/failing-task",
				"-s", "my-resource",
				"--", "sh", "-c",
				"mkdir tmp && mount -o size=50m -t tmpfs swap tmp")
			Eventually(hijackS).Should(gbytes.Say("[1-9]*: build #1, step: my-resource, type: put"))
			fmt.Fprintln(stdin, "3")
			fmt.Fprintln(stdin, "2")
			Eventually(hijackS).Should(gexec.Exit(0))

			By("hijacking into check container for my-resource")
			hijackS = flyHelper.Hijack(
				"-c", pipelineName+"/my-resource",
				"--", "sh", "-c",
				"mkdir tmp && mount -o size=50m -t tmpfs swap tmp")
			Eventually(hijackS).Should(gexec.Exit(0))
		})

		Context("when the custom resource type is not privileged", func() {
			BeforeEach(func() {
				flyHelper.ConfigurePipeline(
					pipelineName,
					"-c", "fixtures/resource-types.yml",
					"-v", "origin-git-server="+originGitServer.URI(),
					"-y", "privileged=false",
				)
			})

			It("should fail running privileged operations in containers", func() {
				watch := flyHelper.TriggerJob(pipelineName, "failing-task")
				<-watch.Exited
				By("hijacking into get container for my-resource")
				hijackS, stdin := flyHelper.HijackInteractive(
					"-j", pipelineName+"/failing-task",
					"-s", "my-resource",
					"--", "sh", "-c",
					"mkdir tmp && mount -o size=50m -t tmpfs swap tmp")
				Eventually(hijackS).Should(gbytes.Say("[1-9]*: build #1, step: my-resource, type: get"))
				fmt.Fprintln(stdin, "1")
				Eventually(hijackS).Should(gexec.Exit(1))

				By("hijacking into put container for my-resource")
				hijackS, stdin = flyHelper.HijackInteractive(
					"-j", pipelineName+"/failing-task",
					"-s", "my-resource",
					"--", "sh", "-c",
					"mkdir tmp && mount -o size=50m -t tmpfs swap tmp")
				Eventually(hijackS).Should(gbytes.Say("[1-9]*: build #1, step: my-resource, type: put"))
				fmt.Fprintln(stdin, "3")
				fmt.Fprintln(stdin, "2")
				Eventually(hijackS).Should(gexec.Exit(1))

				By("hijacking into check container for my-resource")
				hijackS = flyHelper.Hijack(
					"-c", pipelineName+"/my-resource",
					"--", "sh", "-c",
					"mkdir tmp && mount -o size=50m -t tmpfs swap tmp")
				Eventually(hijackS).Should(gexec.Exit(1))
			})
		})
	})

	Context("when resource type named as base resource type", func() {
		BeforeEach(func() {
			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-type-named-as-base-type.yml",
				"-v", "origin-git-server="+originGitServer.URI(),
			)
		})

		It("can use custom resource type named as base resource type", func() {
			watch := flyHelper.Watch(pipelineName, "resource-getter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
		})
	})
})
