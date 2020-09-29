package flaghelpers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

var _ = Describe("PipelineFlag", func() {
	Describe("UnmarshalFlag", func() {

		var pipelineFlag *flaghelpers.PipelineFlag

		BeforeEach(func() {
			pipelineFlag = &flaghelpers.PipelineFlag{}
		})

		Context("when there is only a pipeline name specified", func() {
			It("unmarshal the pipeline name correctly", func() {
				err := pipelineFlag.UnmarshalFlag("some-pipeline")

				Expect(err).ToNot(HaveOccurred())
				Expect(pipelineFlag.Name).To(Equal("some-pipeline"))
				Expect(pipelineFlag.InstanceVars).To(BeNil())
			})
		})

		Context("when a pipeline name specified and instance vars are specified", func() {
			It("unmarshal the pipeline name and instance vars correctly", func() {
				err := pipelineFlag.UnmarshalFlag("some-pipeline/branch:master")

				Expect(err).ToNot(HaveOccurred())
				Expect(pipelineFlag.Name).To(Equal("some-pipeline"))
				Expect(pipelineFlag.InstanceVars).To(Equal(atc.InstanceVars{"branch": "master"}))
			})
		})

		Context("when an instance vars contains the separator character", func() {
			It("unmarshal the pipeline name and instance vars correctly", func() {
				err := pipelineFlag.UnmarshalFlag("some-pipeline/branch:feature/foo")

				Expect(err).ToNot(HaveOccurred())
				Expect(pipelineFlag.Name).To(Equal("some-pipeline"))
				Expect(pipelineFlag.InstanceVars).To(Equal(atc.InstanceVars{"branch": "feature/foo"}))
			})
		})

		Context("when an instance vars is complex", func() {
			It("unmarshal the pipeline name and instance vars correctly", func() {
				err := pipelineFlag.UnmarshalFlag("some-pipeline/foo.bar.baz:1,foo.bar.qux:2,bar.0:1,bar.1:\"2\"")

				Expect(err).ToNot(HaveOccurred())
				Expect(pipelineFlag.Name).To(Equal("some-pipeline"))
				Expect(pipelineFlag.InstanceVars).To(Equal(atc.InstanceVars{
					"bar": []interface{}{1, "2"},
					"foo": map[string]interface{}{
						"bar": map[string]interface{}{
							"baz": 1,
							"qux": 2,
						},
					},
				}))
			})
		})
	})
})
