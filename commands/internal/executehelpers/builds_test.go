package executehelpers_test

import (
	"github.com/concourse/atc"
	. "github.com/concourse/fly/commands/internal/executehelpers"
	"github.com/concourse/go-concourse/concourse/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Builds", func() {
	var fakeClient *fakes.FakeClient
	var config atc.TaskConfig

	BeforeEach(func() {
		fakeClient = new(fakes.FakeClient)

		config = atc.TaskConfig{
			Platform: "shoes",
			Run: atc.TaskRunConfig{
				Path: "./here",
				Args: []string{},
			},
		}
	})

	Context("when tags are provided", func() {
		It("add the tags to the plan", func() {
			tags := []string{"tag", "tag2"}
			_, err := CreateBuild(fakeClient, false, []Input{}, []Output{}, config, tags, "https://target.com")
			Expect(err).ToNot(HaveOccurred())

			plan := fakeClient.CreateBuildArgsForCall(0)
			for index, tag := range (*plan.Do)[1].Task.Tags {
				Expect(tag).To(Equal(tags[index]))
			}
		})
	})

	Context("when tags are not provided", func() {
		It("should not add tags to the plan", func() {
			tags := []string{}
			_, err := CreateBuild(fakeClient, false, []Input{}, []Output{}, config, tags, "https://target.com")
			Expect(err).ToNot(HaveOccurred())

			plan := fakeClient.CreateBuildArgsForCall(0)
			Expect((*plan.Do)[1].Task.Tags).To(BeNil())
		})
	})
})
