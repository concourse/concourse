package db_test

import (
	"context"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Check Lifecycle", func() {
	var (
		lifecycle db.CheckLifecycle
		plan      atc.Plan
	)

	BeforeEach(func() {
		lifecycle = db.NewCheckLifecycle(dbConn)
		plan = atc.Plan{
			ID: "some-plan",
			Check: &atc.CheckPlan{
				Name: "wreck",
			},
		}
	})

	exists := func(b db.Build) bool {
		found, err := b.Reload()
		Expect(err).ToNot(HaveOccurred())
		return found
	}

	createUnfinishedCheck := func(checkable interface {
		CreateBuild(context.Context, bool, atc.Plan) (db.Build, bool, error)
	}, plan atc.Plan) db.Build {
		build, created, err := checkable.CreateBuild(context.Background(), true, plan)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).To(BeTrue())

		return build
	}

	finish := func(build db.Build) {
		err := build.Finish(db.BuildStatusSucceeded)
		Expect(err).ToNot(HaveOccurred())
	}

	createFinishedCheck := func(checkable interface {
		CreateBuild(context.Context, bool, atc.Plan) (db.Build, bool, error)
	}, plan atc.Plan) db.Build {
		build := createUnfinishedCheck(checkable, plan)
		finish(build)
		return build
	}

	It("removes completed check builds when there is a new completed check", func() {
		resourceBuild := createFinishedCheck(defaultResource, plan)
		resourceTypeBuild := createFinishedCheck(defaultResourceType, plan)

		By("attempting to delete completed checks when there are no newer checks")
		err := lifecycle.DeleteCompletedChecks(logger)
		Expect(err).ToNot(HaveOccurred())
		//TODO: fix this test
		//Expect(exists(resourceBuild)).To(BeTrue())
		Expect(exists(resourceTypeBuild)).To(BeTrue())

		By("creating a new check for the resource")
		createFinishedCheck(defaultResource, plan)

		By("deleting completed checks")
		err = lifecycle.DeleteCompletedChecks(logger)
		Expect(err).ToNot(HaveOccurred())

		Expect(exists(resourceBuild)).To(BeFalse())
		Expect(numBuildEventsForCheck(resourceBuild)).To(Equal(0))

		Expect(exists(resourceTypeBuild)).To(BeTrue())

		By("creating a new check for the resource type")
		createFinishedCheck(defaultResourceType, plan)

		By("deleting completed checks")
		err = lifecycle.DeleteCompletedChecks(logger)
		Expect(err).ToNot(HaveOccurred())

		Expect(exists(resourceTypeBuild)).To(BeFalse())
		Expect(numBuildEventsForCheck(resourceTypeBuild)).To(Equal(0))

		By("creating a new check for the prototype")
		createFinishedCheck(defaultPrototype, plan)

		By("deleting completed checks")
		err = lifecycle.DeleteCompletedChecks(logger)
		Expect(err).ToNot(HaveOccurred())
	})

	It("runs check-deletion query in batches", func() {
		db.CheckDeleteBatchSize = 10
		for i := 0; i < 51; i++ {
			createFinishedCheck(defaultResource, plan)
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
		c1 := createUnfinishedCheck(defaultResource, plan)
		c2 := createUnfinishedCheck(defaultResource, plan)

		err := lifecycle.DeleteCompletedChecks(logger)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists(c1)).To(BeTrue())
		Expect(exists(c2)).To(BeTrue())

		By("finishing the first check should allow it to be deleted")
		finish(c1)

		err = lifecycle.DeleteCompletedChecks(logger)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists(c1)).To(BeFalse())
		Expect(exists(c2)).To(BeTrue())

		By("finishing the second check should NOT allow it to be deleted")
		finish(c2)

		err = lifecycle.DeleteCompletedChecks(logger)
		Expect(err).ToNot(HaveOccurred())
		//TODO: fix this test
		//Expect(exists(c2)).To(BeTrue())
	})

	It("deletes all expired checks", func() {
		c1 := createFinishedCheck(defaultResource, plan)
		c2 := createFinishedCheck(defaultResource, plan)

		createFinishedCheck(defaultResource, plan)

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
