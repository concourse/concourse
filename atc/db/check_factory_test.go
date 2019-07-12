package db_test

import (
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckFactory", func() {

	var (
		err                 error
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

	Describe("Check", func() {
		var created, found bool
		var check, foundCheck db.Check

		BeforeEach(func() {
			check, created, err = checkFactory.CreateCheck(
				resourceConfigScope.ID(),
				resourceConfigScope.ResourceConfig().ID(),
				resourceConfigScope.ResourceConfig().OriginBaseResourceType().ID,
				false,
				atc.Plan{Check: &atc.CheckPlan{Name: "some-name", Type: "some-type"}},
			)
			Expect(created).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			foundCheck, found, err = checkFactory.Check(check.ID())
		})

		It("succeeds", func() {
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the db check", func() {
			Expect(foundCheck).To(Equal(check))
		})
	})

	Describe("CreateCheck", func() {
		var created bool
		var check db.Check

		JustBeforeEach(func() {
			check, created, err = checkFactory.CreateCheck(
				resourceConfigScope.ID(),
				resourceConfigScope.ResourceConfig().ID(),
				resourceConfigScope.ResourceConfig().OriginBaseResourceType().ID,
				false,
				atc.Plan{Check: &atc.CheckPlan{Name: "some-name", Type: "some-type"}},
			)
		})

		It("succeeds", func() {
			Expect(created).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the resource check", func() {
			Expect(check.ID()).To(Equal(1))
			Expect(check.Status()).To(Equal(db.CheckStatusStarted))
			Expect(check.Schema()).To(Equal("exec.v2"))
			Expect(check.Plan().Check.Name).To(Equal("some-name"))
			Expect(check.Plan().Check.Type).To(Equal("some-type"))
			Expect(check.CreateTime()).To(BeTemporally("~", time.Now(), time.Second))
			Expect(check.ResourceConfigScopeID()).To(Equal(resourceConfigScope.ID()))
		})

		Context("when a check is already pending", func() {
			BeforeEach(func() {
				_, created, err := checkFactory.CreateCheck(
					resourceConfigScope.ID(),
					resourceConfigScope.ResourceConfig().ID(),
					resourceConfigScope.ResourceConfig().OriginBaseResourceType().ID,
					false,
					atc.Plan{},
				)
				Expect(created).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())
			})

			It("doesn't create a check", func() {
				Expect(created).To(BeFalse())
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("StartedChecks", func() {
		var (
			checks []db.Check
		)

		JustBeforeEach(func() {
			checks, err = checkFactory.StartedChecks()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when looking up the resource check returns no results", func() {
			BeforeEach(func() {
				_, err = dbConn.Exec(`DELETE FROM checks`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("is not found", func() {
				Expect(checks).To(HaveLen(0))
			})
		})

		Context("when looking up the resource check succeeds", func() {
			var check db.Check

			BeforeEach(func() {
				var created bool
				check, created, err = checkFactory.CreateCheck(
					resourceConfigScope.ID(),
					resourceConfigScope.ResourceConfig().ID(),
					resourceConfigScope.ResourceConfig().OriginBaseResourceType().ID,
					false,
					atc.Plan{},
				)
				Expect(created).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the resource checks", func() {
				Expect(checks).To(HaveLen(1))
				Expect(checks[0]).To(Equal(check))
			})
		})
	})

	Describe("Resources", func() {
		var (
			resources []db.Resource
		)

		JustBeforeEach(func() {
			resources, err = checkFactory.Resources()
			Expect(err).NotTo(HaveOccurred())
		})

		It("include resources in return", func() {
			Expect(resources).To(HaveLen(1))
			Expect(resources[0].Name()).To(Equal("some-resource"))
		})

		Context("when the resource is not active", func() {
			BeforeEach(func() {
				_, err = dbConn.Exec(`UPDATE resources SET active = false`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return the resource", func() {
				Expect(resources).To(HaveLen(0))
			})
		})

		Context("when the resource pipeline is paused", func() {
			BeforeEach(func() {
				_, err = dbConn.Exec(`UPDATE pipelines SET paused = true`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return the resource", func() {
				Expect(resources).To(HaveLen(0))
			})
		})
	})

	Describe("ResourceTypes", func() {
		var (
			resourceTypes db.ResourceTypes
		)

		JustBeforeEach(func() {
			resourceTypes, err = checkFactory.ResourceTypes()
			Expect(err).NotTo(HaveOccurred())
		})

		It("include resource types in return", func() {
			Expect(resourceTypes).To(HaveLen(1))
			Expect(resourceTypes[0].Name()).To(Equal("some-type"))
		})

		Context("when the resource type is not active", func() {
			BeforeEach(func() {
				_, err = dbConn.Exec(`UPDATE resource_types SET active = false`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return the resource type", func() {
				Expect(resourceTypes).To(HaveLen(0))
			})
		})
	})
})
