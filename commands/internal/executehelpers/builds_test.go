package executehelpers_test

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/deprecated"
	. "github.com/concourse/fly/commands/internal/executehelpers"
	"github.com/concourse/go-concourse/concourse/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Builds", func() {
	var requester *deprecated.AtcRequester
	var fakeClient *fakes.FakeClient
	var config atc.TaskConfig

	BeforeEach(func() {
		requester = deprecated.NewAtcRequester("foo", &http.Client{})
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
			_, err := CreateBuild(requester, fakeClient, false, []Input{}, []Output{}, config, tags, "https://target.com")
			Expect(err).ToNot(HaveOccurred())

			plan := fakeClient.CreateBuildArgsForCall(0)
			for index, tag := range plan.OnSuccess.Next.Task.Tags {
				Expect(tag).To(Equal(tags[index]))
			}
		})
	})

	Context("when tags are not provided", func() {
		It("should not add tags to the plan", func() {
			tags := []string{}
			_, err := CreateBuild(requester, fakeClient, false, []Input{}, []Output{}, config, tags, "https://target.com")
			Expect(err).ToNot(HaveOccurred())

			plan := fakeClient.CreateBuildArgsForCall(0)
			Expect(plan.OnSuccess.Next.Task.Tags).To(BeNil())
		})
	})
})
