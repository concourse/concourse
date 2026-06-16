package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with nested volume mounts", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/volume-mounting.yml")
	})

	It("procceds through the plan with input under output mounts", func(ctx SpecContext) {
		watch := fly("trigger-job", "-j", inPipeline("input-under-output"), "-w")
		Expect(watch).To(gexec.Exit(0))
		Expect(watch).To(gbytes.Say("some-resource"))
	}, DefaultSpecTimeout)

	It("procceds through the plan with input under input mounts", func(ctx SpecContext) {
		sess := fly("trigger-job", "-j", inPipeline("input-under-input"), "-w")
		Expect(sess).To(gexec.Exit(0))
		Expect(sess).To(gbytes.Say("helloworld"))
	}, DefaultSpecTimeout)

	It("procceds through the plan having output being mapped to dot and input within", func(ctx SpecContext) {
		sess := fly("trigger-job", "-j", inPipeline("output-with-dot-with-input-within"), "-w")
		Expect(sess).To(gexec.Exit(0))
		Expect(sess).To(gbytes.Say("bar"))
	}, DefaultSpecTimeout)

	It("procceds through the plan with output under input mounts", func(ctx SpecContext) {
		sess := fly("trigger-job", "-j", inPipeline("output-under-input"), "-w")
		Expect(sess).To(gexec.Exit(0))
		Expect(sess).To(gbytes.Say("hello"))
	}, DefaultSpecTimeout)

	It("procceds through the plan with input same as output mounts", func(ctx SpecContext) {
		sess := fly("trigger-job", "-j", inPipeline("input-same-output"), "-w")
		Expect(sess).To(gexec.Exit(0))
		Expect(sess).To(gbytes.Say("hello"))
	}, DefaultSpecTimeout)

	It("procceds through the plan with input and output having the same path but a different name", func(ctx SpecContext) {
		sess := fly("trigger-job", "-j", inPipeline("input-output-same-path-diff-name"), "-w")
		Expect(sess).To(gexec.Exit(0))
	}, DefaultSpecTimeout)
})

var _ = Describe("Containers with nonroot users have their volume mounts chown'd", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/nonroot-chowning.yml")
	})

	It("volumes are by default owned by root", func(ctx SpecContext) {
		sess := fly("trigger-job", "-j", inPipeline("root"), "-w")
		Expect(sess).To(gexec.Exit(0))
	}, DefaultSpecTimeout)

	It("volumes are by default owned by root (privileged)", func(ctx SpecContext) {
		sess := fly("trigger-job", "-j", inPipeline("root-privileged"), "-w")
		Expect(sess).To(gexec.Exit(0))
	}, DefaultSpecTimeout)

	It("volumes owned by nonroot when user is nonroot", func(ctx SpecContext) {
		sess := fly("trigger-job", "-j", inPipeline("nonroot"), "-w")
		Expect(sess).To(gexec.Exit(0))
	}, DefaultSpecTimeout)

	It("volumes owned by nonroot when user is nonroot (privileged)", func(ctx SpecContext) {
		sess := fly("trigger-job", "-j", inPipeline("nonroot-privileged"), "-w")
		Expect(sess).To(gexec.Exit(0))
	}, DefaultSpecTimeout)

	It("volume passed between steps is chown'd by different nonroot users", func(ctx SpecContext) {
		sess := fly("trigger-job", "-j", inPipeline("different-users"), "-w")
		Expect(sess).To(gexec.Exit(0))
	}, DefaultSpecTimeout)

	It("passing nonroot volume to put step works", func(ctx SpecContext) {
		sess := fly("trigger-job", "-j", inPipeline("put"), "-w")
		Expect(sess).To(gexec.Exit(0))
	}, DefaultSpecTimeout)

	It("volumed owned by nonroot is passed as-is to step running as root", func(ctx SpecContext) {
		sess := fly("trigger-job", "-j", inPipeline("nonroot-to-root"), "-w")
		Expect(sess).To(gexec.Exit(0))
	}, DefaultSpecTimeout)

	It("parent volume is correctly COW'd and only chown'd for the nonroot task", func(ctx SpecContext) {
		sess := fly("trigger-job", "-j", inPipeline("nonroot-and-root-same-parent-volume"), "-w")
		Expect(sess).To(gexec.Exit(0))
	}, DefaultSpecTimeout)
})
