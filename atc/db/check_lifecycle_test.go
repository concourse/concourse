package db_test

import (
	"context"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Check Lifecycle", func() {
	var (
		lifecycle                  db.CheckLifecycle
		plan                       atc.Plan
		scopeOfDefaultResource     db.ResourceConfigScope
		scopeOfDefaultResourceType db.ResourceConfigScope
		scopeOfDefaultPrototype    db.ResourceConfigScope
	)

	BeforeEach(func() {
		lifecycle = db.NewCheckLifecycle(dbConn)
		plan = atc.Plan{
			ID: "some-plan",
			Check: &atc.CheckPlan{
				Name: "wreck",
			},
		}

		resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(defaultResource.Type(), defaultResource.Source(), nil)
		Expect(err).ToNot(HaveOccurred())
		scopeOfDefaultResource, err = resourceConfig.FindOrCreateScope(intptr(defaultResource.ID()))
		Expect(err).ToNot(HaveOccurred())

		resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(defaultResourceType.Type(), defaultResourceType.Source(), nil)
		Expect(err).ToNot(HaveOccurred())
		scopeOfDefaultResourceType, err = resourceConfig.FindOrCreateScope(nil)
		Expect(err).ToNot(HaveOccurred())

		resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(defaultPrototype.Type(), defaultPrototype.Source(), nil)
		Expect(err).ToNot(HaveOccurred())
		scopeOfDefaultPrototype, err = resourceConfig.FindOrCreateScope(nil)
	})

	Context("DB build", func() {
		exists := func(b db.Build) bool {
			found, err := b.Reload()
			Expect(err).ToNot(HaveOccurred())
			return found
		}

		createUnfinishedCheck := func(checkable interface {
			CreateBuild(context.Context, bool, atc.Plan) (db.Build, bool, error)
		}, scope db.ResourceConfigScope, plan atc.Plan) db.Build {
			build, created, err := checkable.CreateBuild(context.Background(), true, plan)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(BeTrue())
			_, err = scope.UpdateLastCheckStartTime(build.ID(), nil)
			Expect(err).ToNot(HaveOccurred())
			return build
		}

		finish := func(build db.Build, scope db.ResourceConfigScope) {
			err := build.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())
			_, err = scope.UpdateLastCheckEndTime(true)
			Expect(err).ToNot(HaveOccurred())
		}

		createFinishedCheck := func(checkable interface {
			CreateBuild(context.Context, bool, atc.Plan) (db.Build, bool, error)
		}, scope db.ResourceConfigScope, plan atc.Plan) db.Build {
			build := createUnfinishedCheck(checkable, scope, plan)
			finish(build, scope)
			return build
		}

		It("removes completed check builds when there is a new completed check", func() {
			resourceBuild := createFinishedCheck(defaultResource, scopeOfDefaultResource, plan)
			resourceTypeBuild := createFinishedCheck(defaultResourceType, scopeOfDefaultResourceType, plan)

			By("attempting to delete completed checks when there are no newer checks")
			err := lifecycle.DeleteCompletedChecks(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists(resourceBuild)).To(BeTrue())
			Expect(exists(resourceTypeBuild)).To(BeTrue())

			By("creating a new check for the resource")
			createFinishedCheck(defaultResource, scopeOfDefaultResource, plan)

			By("deleting completed checks")
			err = lifecycle.DeleteCompletedChecks(logger)
			Expect(err).ToNot(HaveOccurred())

			Expect(exists(resourceBuild)).To(BeFalse())
			Expect(numBuildEventsForCheck(resourceBuild)).To(Equal(0))

			Expect(exists(resourceTypeBuild)).To(BeTrue())

			By("creating a new check for the resource type")
			createFinishedCheck(defaultResourceType, scopeOfDefaultResourceType, plan)

			By("deleting completed checks")
			err = lifecycle.DeleteCompletedChecks(logger)
			Expect(err).ToNot(HaveOccurred())

			Expect(exists(resourceTypeBuild)).To(BeFalse())
			Expect(numBuildEventsForCheck(resourceTypeBuild)).To(Equal(0))

			By("creating a new check for the prototype")
			createFinishedCheck(defaultPrototype, scopeOfDefaultPrototype, plan)

			By("deleting completed checks")
			err = lifecycle.DeleteCompletedChecks(logger)
			Expect(err).ToNot(HaveOccurred())
		})

		It("runs check-deletion query in batches", func() {
			db.CheckDeleteBatchSize = 10
			for i := 0; i < 51; i++ {
				build := createFinishedCheck(defaultResource, scopeOfDefaultResource, plan)
				scopeOfDefaultResourceType.UpdateLastCheckStartTime(build.ID(), nil)
			}

			By("deleting completed checks")
			err := lifecycle.DeleteCompletedChecks(logger)
			Expect(err).ToNot(HaveOccurred())

			By("verifying all checks but the latest were deleted")
			var numChecksRemaining int
			err = dbConn.QueryRow(`SELECT COUNT(*) FROM builds WHERE resource_id = $1`, defaultResource.ID()).Scan(&numChecksRemaining)
			Expect(err).ToNot(HaveOccurred())

			Expect(numChecksRemaining).To(Equal(1))
		})

		It("ignores incomplete checks", func() {
			c1 := createUnfinishedCheck(defaultResource, scopeOfDefaultResource, plan)
			c2 := createUnfinishedCheck(defaultResource, scopeOfDefaultResource, plan)

			err := lifecycle.DeleteCompletedChecks(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists(c1)).To(BeTrue())
			Expect(exists(c2)).To(BeTrue())

			By("finishing the first check should allow it to be deleted")
			finish(c1, scopeOfDefaultResource)

			err = lifecycle.DeleteCompletedChecks(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists(c1)).To(BeFalse())
			Expect(exists(c2)).To(BeTrue())

			By("finishing the second check should NOT allow it to be deleted")
			finish(c2, scopeOfDefaultResource)
			scopeOfDefaultResource.UpdateLastCheckStartTime(c2.ID(), nil)

			err = lifecycle.DeleteCompletedChecks(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists(c2)).To(BeTrue())
		})

		It("deletes all expired checks", func() {
			c1 := createFinishedCheck(defaultResource, scopeOfDefaultResource, plan)
			c2 := createFinishedCheck(defaultResource, scopeOfDefaultResource, plan)

			createFinishedCheck(defaultResource, scopeOfDefaultResource, plan)

			err := lifecycle.DeleteCompletedChecks(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists(c1)).To(BeFalse())
			Expect(exists(c2)).To(BeFalse())
		})

		It("ignores job builds", func() {
			build, err := defaultJob.CreateBuild("foo")
			Expect(err).ToNot(HaveOccurred())

			err = build.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			By("creating a new build for the same job")
			_, err = defaultJob.CreateBuild("foo")
			Expect(err).ToNot(HaveOccurred())

			err = lifecycle.DeleteCompletedChecks(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists(build)).To(BeTrue())
		})
	})

	Context("In-memory check build", func() {
		var build db.Build
		var seqGen util.SequenceGenerator

		BeforeEach(func() {
			seqGen = util.NewSequenceGenerator(1)

			var err error
			build, err = defaultResource.CreateInMemoryBuild(context.Background(), plan, seqGen)
			Expect(err).ToNot(HaveOccurred())

			err = build.OnCheckBuildStart()
			Expect(err).ToNot(HaveOccurred())

			_, err = scopeOfDefaultResource.UpdateLastCheckStartTime(build.ID(), nil)
			Expect(err).ToNot(HaveOccurred())

			err = build.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())
		})

		It("cleanup check events for", func() {
			By("when there is no newer build, events should not be cleaned")
			err := lifecycle.DeleteCompletedChecks(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(numBuildEventsForCheck(build)).To(Equal(2)) // should be status and finish event

			By("when there is a newer build, old events should be cleaned up")
			newBuild, err := defaultResource.CreateInMemoryBuild(context.Background(), plan, seqGen)
			Expect(err).ToNot(HaveOccurred())
			err = newBuild.OnCheckBuildStart()
			Expect(err).ToNot(HaveOccurred())
			_, err = scopeOfDefaultResource.UpdateLastCheckStartTime(newBuild.ID(), nil)
			Expect(err).ToNot(HaveOccurred())
			err = lifecycle.DeleteCompletedChecks(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(numBuildEventsForCheck(build)).To(Equal(0))
		})
	})
})

func numBuildEventsForCheck(check db.Build) int {
	var count int
	err := psql.Select("COUNT(*)").
		From("check_build_events").
		Where(sq.Eq{"build_id": check.ID()}).
		RunWith(dbConn).
		QueryRow().
		Scan(&count)
	Expect(err).ToNot(HaveOccurred())
	return count
}
