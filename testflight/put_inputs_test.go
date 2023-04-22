package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A put step with inputs", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/put-inputs.yml")
	})

	Context("when specific inputs are specified", func() {
		It("attaches only the specified inputs to the put container", func() {
			watch := spawnFly("trigger-job", "-j", inPipeline("job-using-specified-inputs"), "-w")
			<-watch.Exited

			interceptS := fly("intercept", "-j", inPipeline("job-using-specified-inputs"), "-s", "some-resource", "--step-type", "put", "--", "ls")
			Expect(interceptS).To(gbytes.Say("specified-input"))
			Expect(string(interceptS.Out.Contents())).ToNot(ContainSubstring("all-input"))
		})
	})

	Context("when it uses all inputs", func() {
		It("attached all inputs to the put container", func() {
			watch := spawnFly("trigger-job", "-j", inPipeline("job-using-all-inputs"), "-w")
			<-watch.Exited

			interceptS := fly("intercept", "-j", inPipeline("job-using-all-inputs"), "-s", "some-resource", "--step-type", "put", "--", "ls")
			Expect(string(interceptS.Out.Contents())).To(ContainSubstring("all-input"))
			Expect(string(interceptS.Out.Contents())).To(ContainSubstring("specified-input"))
		})
	})

	Context("when an empty slice of inputs are specified", func() {
		It("no inputs are attached to the put container", func() {
			watch := spawnFly("trigger-job", "-j", inPipeline("job-using-empty-inputs"), "-w")
			<-watch.Exited

			interceptS := fly("intercept", "-j", inPipeline("job-using-empty-inputs"), "-s", "some-resource", "--step-type", "put", "--", "ls")
			Expect(string(interceptS.Out.Contents())).ToNot(ContainSubstring("all-input"))
			Expect(string(interceptS.Out.Contents())).ToNot(ContainSubstring("specified-input"))
		})
	})

	Context("when no inputs are specified", func() {
		It("attached all inputs to the put container", func() {
			watch := spawnFly("trigger-job", "-j", inPipeline("job-using-no-inputs"), "-w")
			<-watch.Exited

			interceptS := fly("intercept", "-j", inPipeline("job-using-no-inputs"), "-s", "some-resource", "--step-type", "put", "--", "ls")
			Expect(string(interceptS.Out.Contents())).To(ContainSubstring("all-input"))
			Expect(string(interceptS.Out.Contents())).To(ContainSubstring("specified-input"))
		})
	})

	Context("when inputs are detected", func() {
		Context("when params are only strings", func() {
			It("attaches only the detected inputs to the put container", func() {
				watch := spawnFly("trigger-job", "-j", inPipeline("job-using-detect-inputs-simple"), "-w")
				<-watch.Exited

				interceptS := fly("intercept", "-j", inPipeline("job-using-detect-inputs-simple"), "-s", "some-resource", "--step-type", "put", "--", "ls")
				Expect(string(interceptS.Out.Contents())).To(ContainSubstring("specified-input"))
				Expect(string(interceptS.Out.Contents())).ToNot(ContainSubstring("all-input"))
			})
		})

		Context("when params have slices and maps", func() {
			It("attaches the inputs detected in the slices", func() {
				watch := spawnFly("trigger-job", "-j", inPipeline("job-using-detect-inputs-complex"), "-w")
				<-watch.Exited

				interceptS := fly("intercept", "-j", inPipeline("job-using-detect-inputs-complex"), "-s", "some-resource", "--step-type", "put", "--", "ls")
				Expect(string(interceptS.Out.Contents())).To(ContainSubstring("specified-input"))
				Expect(string(interceptS.Out.Contents())).To(ContainSubstring("map-input"))
				Expect(string(interceptS.Out.Contents())).ToNot(ContainSubstring("all-input"))
				Expect(string(interceptS.Out.Contents())).ToNot(ContainSubstring("does-not-exist"))
				Expect(string(interceptS.Out.Contents())).ToNot(ContainSubstring("does-not-exist-too"))
				Expect(string(interceptS.Out.Contents())).ToNot(ContainSubstring("123456"))
			})
		})
	})
})
