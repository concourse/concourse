package db_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc/db"
	gocache "github.com/patrickmn/go-cache"
)

var _ = Describe("VersionsDB", func() {
	var vdb db.VersionsDB
	var pageLimit int
	var cache *gocache.Cache

	var ctx context.Context

	BeforeEach(func() {
		pageLimit = 5
		cache = gocache.New(-1, -1)
		vdb = db.NewVersionsDB(dbConn, pageLimit, cache)

		ctx = context.Background()
	})

	AfterEach(func() {
		cache.Flush()
	})

	Describe("SuccessfulBuilds", func() {
		var paginatedBuilds db.PaginatedBuilds

		JustBeforeEach(func() {
			paginatedBuilds = vdb.SuccessfulBuilds(ctx, defaultJob.ID())
		})

		Context("with one build", func() {
			var build db.Build

			BeforeEach(func() {
				var err error
				build, err = defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				err = build.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the build and finishes", func() {
				buildID, ok, err := paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())
				Expect(buildID).To(BeZero())
			})
		})

		Context("with the same number of builds as the page limit", func() {
			var builds []db.Build

			BeforeEach(func() {
				builds = []db.Build{}

				for i := 0; i < pageLimit; i++ {
					build, err := defaultJob.CreateBuild()
					Expect(err).ToNot(HaveOccurred())

					err = build.Finish(db.BuildStatusSucceeded)
					Expect(err).ToNot(HaveOccurred())

					builds = append(builds, build)
				}
			})

			It("returns all of the builds, newest first, and then finishes", func() {
				for i := pageLimit - 1; i >= 0; i-- {
					buildID, ok, err := paginatedBuilds.Next(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(BeTrue())
					Expect(buildID).To(Equal(builds[i].ID()))
				}

				buildID, ok, err := paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())
				Expect(buildID).To(BeZero())
			})
		})

		Context("with a page of filler and then rerun builds created after their original builds", func() {
			var build1Succeeded db.Build
			var build2Failed db.Build
			var build3Succeeded db.Build
			var build4Rerun2Succeeded db.Build
			var build5Rerun2Succeeded db.Build
			var build6Succeeded db.Build
			var fillerBuilds []db.Build

			BeforeEach(func() {
				var err error
				build1Succeeded, err = defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
				err = build1Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				build2Failed, err = defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
				err = build2Failed.Finish(db.BuildStatusFailed)
				Expect(err).ToNot(HaveOccurred())

				build3Succeeded, err = defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
				err = build3Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				build4Rerun2Succeeded, err = defaultJob.RerunBuild(build2Failed)
				Expect(err).ToNot(HaveOccurred())
				err = build4Rerun2Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				build5Rerun2Succeeded, err = defaultJob.RerunBuild(build2Failed)
				Expect(err).ToNot(HaveOccurred())
				err = build5Rerun2Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				build6Succeeded, err = defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
				err = build6Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				for i := 0; i < pageLimit; i++ {
					build, err := defaultJob.CreateBuild()
					Expect(err).ToNot(HaveOccurred())

					err = build.Finish(db.BuildStatusSucceeded)
					Expect(err).ToNot(HaveOccurred())

					fillerBuilds = append(fillerBuilds, build)
				}
			})

			It("returns all of the builds, newest first, with reruns relative to original build's order, and then finishes", func() {
				for i := len(fillerBuilds) - 1; i >= 0; i-- {
					buildID, ok, err := paginatedBuilds.Next(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(BeTrue())
					Expect(buildID).To(Equal(fillerBuilds[i].ID()))
				}

				buildID, ok, err := paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build6Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build3Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build5Rerun2Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build4Rerun2Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build1Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())
				Expect(buildID).To(BeZero())
			})
		})

		Context("with rerun builds created after their original builds", func() {
			var build1Succeeded db.Build
			var build2Failed db.Build
			var build3Succeeded db.Build
			var build4Rerun2Succeeded db.Build
			var build5Rerun2Succeeded db.Build
			var build6Succeeded db.Build

			BeforeEach(func() {
				var err error
				build1Succeeded, err = defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
				err = build1Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				build2Failed, err = defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
				err = build2Failed.Finish(db.BuildStatusFailed)
				Expect(err).ToNot(HaveOccurred())

				build3Succeeded, err = defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
				err = build3Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				build4Rerun2Succeeded, err = defaultJob.RerunBuild(build2Failed)
				Expect(err).ToNot(HaveOccurred())
				err = build4Rerun2Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				build5Rerun2Succeeded, err = defaultJob.RerunBuild(build2Failed)
				Expect(err).ToNot(HaveOccurred())
				err = build5Rerun2Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				build6Succeeded, err = defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
				err = build6Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns all of the builds, newest first, with reruns relative to original build's order, and then finishes", func() {
				buildID, ok, err := paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build6Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build3Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build5Rerun2Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build4Rerun2Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build1Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())
				Expect(buildID).To(BeZero())
			})
		})

		Context("with a rerun build on the page limit boundary", func() {
			var build1Failed db.Build
			var fillerBuilds []db.Build
			var build6Rerun1Succeeded db.Build

			BeforeEach(func() {
				var err error
				build1Failed, err = defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
				err = build1Failed.Finish(db.BuildStatusFailed)
				Expect(err).ToNot(HaveOccurred())

				fillerBuilds = []db.Build{}

				for i := 0; i < pageLimit-1; i++ {
					build, err := defaultJob.CreateBuild()
					Expect(err).ToNot(HaveOccurred())

					err = build.Finish(db.BuildStatusSucceeded)
					Expect(err).ToNot(HaveOccurred())

					fillerBuilds = append(fillerBuilds, build)
				}

				build6Rerun1Succeeded, err = defaultJob.RerunBuild(build1Failed)
				Expect(err).ToNot(HaveOccurred())
				err = build6Rerun1Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())
			})

			It("finishes after the rerun", func() {
				for i := len(fillerBuilds) - 1; i >= 0; i-- {
					buildID, ok, err := paginatedBuilds.Next(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(BeTrue())
					Expect(buildID).To(Equal(fillerBuilds[i].ID()))
				}

				buildID, ok, err := paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build6Rerun1Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())
				Expect(buildID).To(BeZero())
			})
		})

		Context("with multiple reruns of the same build crossing the page limit boundary", func() {
			var build1Failed db.Build
			var fillerBuilds []db.Build
			var build6Rerun1Succeeded db.Build
			var build7Rerun1Succeeded db.Build
			var build8Rerun1Succeeded db.Build

			BeforeEach(func() {
				var err error
				build1Failed, err = defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
				err = build1Failed.Finish(db.BuildStatusFailed)
				Expect(err).ToNot(HaveOccurred())

				fillerBuilds = []db.Build{}

				for i := 0; i < pageLimit-1; i++ {
					build, err := defaultJob.CreateBuild()
					Expect(err).ToNot(HaveOccurred())

					err = build.Finish(db.BuildStatusSucceeded)
					Expect(err).ToNot(HaveOccurred())

					fillerBuilds = append(fillerBuilds, build)
				}

				build6Rerun1Succeeded, err = defaultJob.RerunBuild(build1Failed)
				Expect(err).ToNot(HaveOccurred())
				err = build6Rerun1Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				build7Rerun1Succeeded, err = defaultJob.RerunBuild(build1Failed)
				Expect(err).ToNot(HaveOccurred())
				err = build7Rerun1Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				build8Rerun1Succeeded, err = defaultJob.RerunBuild(build1Failed)
				Expect(err).ToNot(HaveOccurred())
				err = build8Rerun1Succeeded.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns all builds, and then all three reruns, and finishes", func() {
				for i := len(fillerBuilds) - 1; i >= 0; i-- {
					buildID, ok, err := paginatedBuilds.Next(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(BeTrue())
					Expect(buildID).To(Equal(fillerBuilds[i].ID()))
				}

				buildID, ok, err := paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build8Rerun1Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build7Rerun1Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(buildID).To(Equal(build6Rerun1Succeeded.ID()))

				buildID, ok, err = paginatedBuilds.Next(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())
				Expect(buildID).To(BeZero())
			})
		})
	})
})
