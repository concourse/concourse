package getbuilds_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"
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

		pagination := concourse.Pagination{
			Previous: &concourse.Page{Until: 2, Limit: 1},
			Next:     &concourse.Page{Since: 2, Limit: 1},
		}

		fakeClient.BuildsReturns(builds, pagination, nil)

		templateData, err := FetchTemplateData(fakeClient, concourse.Page{})
		Expect(err).NotTo(HaveOccurred())

		Expect(templateData.Builds[0].ID).To(Equal(6))
		Expect(templateData.Builds).To(BeAssignableToTypeOf([]PresentedBuild{}))

		Expect(templateData.Pagination).To(Equal(pagination))
	})

	It("returns an error if fetching from the database fails", func() {
		fakeClient.BuildsReturns(nil, concourse.Pagination{}, errors.New("disaster"))

		_, err := FetchTemplateData(fakeClient, concourse.Page{})
		Expect(err).To(HaveOccurred())
	})
})
