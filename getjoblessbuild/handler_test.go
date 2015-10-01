package getjoblessbuild_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	dbfakes "github.com/concourse/atc/db/fakes"

	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/web/getjoblessbuild"
	"github.com/concourse/atc/web/getjoblessbuild/fakes"
)

var _ = Describe("Handler", func() {
	Describe("creating the Template Data", func() {
		var fakeDB *fakes.FakeBuildDB
		var fakeConfigDB *dbfakes.FakeConfigDB

		BeforeEach(func() {
			fakeDB = new(fakes.FakeBuildDB)
			fakeConfigDB = new(dbfakes.FakeConfigDB)
		})

		It("queries the database by id to get a build", func() {
			build := db.Build{
				ID: 3,
			}

			fakeDB.GetBuildReturns(build, true, nil)

			templateData, err := FetchTemplateData("3", fakeDB, fakeConfigDB)
			Expect(err).NotTo(HaveOccurred())

			Expect(templateData.Build.ID).To(Equal(3))
			Expect(templateData.Build).To(BeAssignableToTypeOf(db.Build{}))
		})

		It("errors if the db returns an error", func() {
			fakeDB.GetBuildReturns(db.Build{}, false, errors.New("disaster"))

			_, err := FetchTemplateData("1", fakeDB, fakeConfigDB)
			Expect(err).To(HaveOccurred())
		})

		It("errors if the build ID is not an integer", func() {
			_, err := FetchTemplateData("not-a-number", fakeDB, fakeConfigDB)
			Expect(err).To(MatchError(ErrInvalidBuildID))
		})
	})
})
