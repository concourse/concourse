package getbuilds_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	cfakes "github.com/concourse/go-concourse/concourse/fakes"

	. "github.com/concourse/atc/web/getbuilds"
)

var _ = Describe("FetchTemplateData", func() {
	var fakeClient *cfakes.FakeClient

	BeforeEach(func() {
		fakeClient = new(cfakes.FakeClient)
	})

	It("queries the database for a list of all builds", func() {
		builds := []atc.Build{
			atc.Build{
				ID: 6,
			},
		}

		fakeClient.AllBuildsReturns(builds, nil)

		templateData, err := FetchTemplateData(fakeClient)
		Expect(err).NotTo(HaveOccurred())

		Expect(templateData.Builds[0].ID).To(Equal(6))
		Expect(templateData.Builds).To(BeAssignableToTypeOf([]PresentedBuild{}))
	})

	It("returns an error if fetching from the database fails", func() {
		fakeClient.AllBuildsReturns(nil, errors.New("disaster"))

		_, err := FetchTemplateData(fakeClient)
		Expect(err).To(HaveOccurred())
	})
})
