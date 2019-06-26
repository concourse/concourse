package db_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Check", func() {
	var (
		err                 error
		created             bool
		check               db.Check
		resourceConfigScope db.ResourceConfigScope
	)

	BeforeEach(func() {

		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		brt := db.BaseResourceType{
			Name: "some-base-resource-type",
		}

		_, err = brt.FindOrCreate(setupTx, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(setupTx.Commit()).To(Succeed())

		resourceConfigScope, err = defaultResource.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
		Expect(err).NotTo(HaveOccurred())

	})

	JustBeforeEach(func() {
		check, created, err = checkFactory.CreateCheck(
			resourceConfigScope.ID(),
			resourceConfigScope.ResourceConfig().ID(),
			resourceConfigScope.ResourceConfig().OriginBaseResourceType().ID,
			atc.Plan{},
		)
		Expect(created).To(BeTrue())
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Check", func() {
		It("succeeds", func() {
			Expect(check.Status()).To(Equal(db.CheckStatusStarted))
			Expect(check.ResourceConfigScopeID()).To(Equal(resourceConfigScope.ID()))
			Expect(check.ResourceConfigID()).To(Equal(resourceConfigScope.ResourceConfig().ID()))
			Expect(check.BaseResourceTypeID()).To(Equal(resourceConfigScope.ResourceConfig().OriginBaseResourceType().ID))
		})
	})
})
