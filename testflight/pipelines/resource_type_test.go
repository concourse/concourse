package pipelines_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Configuring a resource type in a pipeline config", func() {
	Context("with custom resource types", func() {
		BeforeEach(func() {
			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-types.yml",
			)
		})

		It("can use custom resource types for 'get', 'put', and task 'image_resource's", func() {
			watch := flyHelper.TriggerJob(pipelineName, "resource-getter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("fetched version: hello-from-custom-type"))

			watch = flyHelper.TriggerJob(pipelineName, "resource-putter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("pushing version: some-pushed-version"))

			watch = flyHelper.TriggerJob(pipelineName, "resource-imgur")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("MIRRORED_VERSION=image-version"))
		})

		It("can check for resources using a custom type", func() {
			checkResource := flyHelper.CheckResource("-r", fmt.Sprintf("%s/my-resource", pipelineName))
			<-checkResource.Exited
			Expect(checkResource.ExitCode()).To(Equal(0))
			Expect(checkResource).To(gbytes.Say("checked 'my-resource'"))
		})

		XIt("should be able to run privileged operations in 'check', 'get' and 'put' steps", func() {
			watch := flyHelper.TriggerJob(pipelineName, "failing-task")
			<-watch.Exited
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

		XContext("when the custom resource type is not privileged", func() {
			BeforeEach(func() {
				flyHelper.ConfigurePipeline(
					pipelineName,
					"-c", "fixtures/resource-types.yml",
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

	Context("with custom resource types that have params", func() {
		BeforeEach(func() {
			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-types-with-params.yml",
			)
		})

		It("can use a custom resource with parameters", func() {
			watch := flyHelper.TriggerJob(pipelineName, "resource-test")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("mirror"))
		})
	})

	Context("when resource type named as base resource type", func() {
		BeforeEach(func() {
			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-type-named-as-base-type.yml",
			)
		})

		It("can use custom resource type named as base resource type", func() {
			watch := flyHelper.TriggerJob(pipelineName, "resource-getter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("mirror-mirror"))
		})
	})
})
