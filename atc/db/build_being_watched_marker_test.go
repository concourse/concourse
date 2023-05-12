package db_test

import (
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/v3/lagerctx"
	"context"
	"fmt"
	"github.com/concourse/concourse/atc/db"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BeingWatchedBuildEventChannelMap", func() {
	It("should marked channel be queried", func() {
		m := db.NewBeingWatchedBuildEventChannelMap()

		m.Mark("channel-1", time.Now())
		Expect(m.BeingWatched("channel-1")).To(BeTrue())
		Expect(m.BeingWatched("channel-2")).To(BeFalse())

		// Condition func always return false, thus no entry should be deleted.
		m.Clean(func(k string, _ time.Time) bool { return false })
		Expect(m.BeingWatched("channel-1")).To(BeTrue())

		// Condition func always return true, thus no entry should be deleted.
		m.Clean(func(k string, _ time.Time) bool { return true })
		Expect(m.BeingWatched("channel-1")).To(BeFalse())
	})
})

var _ = Describe("BuildBeingWatchedMarker", func() {
	var (
		bm        *db.BuildBeingWatchedMarker
		fakeClock *fakeclock.FakeClock
	)

	BeforeEach(func() {
		var err error
		fakeClock = fakeclock.NewFakeClock(time.Now())
		bm, err = db.NewBuildBeingWatchedMarker(logger, dbConn, db.DefaultBuildBeingWatchedMarkDuration, fakeClock)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		bm.Drain(context.Background())
	})

	Context("Cleanup", func() {
		BeforeEach(func() {
			err := db.MarkBuildAsBeingWatched(dbConn, "build_events_10000")
			Expect(err).NotTo(HaveOccurred())

			err = db.MarkBuildAsBeingWatched(dbConn, "build_events_20000")
			Expect(err).NotTo(HaveOccurred())

			// As MarkBuildAsBeingWatched is async, sleep a second to wait for mark done.
			time.Sleep(time.Second)
		})

		JustBeforeEach(func() {
			err := bm.Run(lagerctx.NewContext(context.Background(), logger))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not delete before retain duration expired ", func() {
			Expect(db.NewBeingWatchedBuildEventChannelMap().BeingWatched("build_events_10000")).To(BeTrue())
			Expect(db.NewBeingWatchedBuildEventChannelMap().BeingWatched("build_events_20000")).To(BeTrue())
		})

		Context("when reach to retain duration expired", func() {
			BeforeEach(func() {
				fakeClock.Increment(db.DefaultBuildBeingWatchedMarkDuration + time.Second)
			})

			It("should delete before retain duration expired ", func() {
				Expect(db.NewBeingWatchedBuildEventChannelMap().BeingWatched("build_events_10000")).To(BeFalse())
				Expect(db.NewBeingWatchedBuildEventChannelMap().BeingWatched("build_events_20000")).To(BeFalse())
			})
		})
	})

	Context("Cleanup with existing builds", func() {
		var (
			build1   db.Build
			build2   db.Build
			channel1 string
			channel2 string
			err      error
		)
		BeforeEach(func() {
			build1, err = defaultTeam.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			build2, err = defaultTeam.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			channel1 = fmt.Sprintf("build_events_%d", build1.ID())
			channel2 = fmt.Sprintf("build_events_%d", build2.ID())

			err := db.MarkBuildAsBeingWatched(dbConn, channel1)
			Expect(err).NotTo(HaveOccurred())

			err = db.MarkBuildAsBeingWatched(dbConn, channel2)
			Expect(err).NotTo(HaveOccurred())

			// As MarkBuildAsBeingWatched is async, sleep a second to wait for mark done.
			time.Sleep(time.Second)
		})

		JustBeforeEach(func() {
			err := bm.Run(lagerctx.NewContext(context.Background(), logger))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not delete before retain duration expired ", func() {
			Expect(db.NewBeingWatchedBuildEventChannelMap().BeingWatched(channel1)).To(BeTrue())
			Expect(db.NewBeingWatchedBuildEventChannelMap().BeingWatched(channel2)).To(BeTrue())
		})

		Context("when retain duration expired", func() {
			BeforeEach(func() {
				fakeClock.Increment(db.DefaultBuildBeingWatchedMarkDuration + time.Second)
			})

			It("should not delete", func() {
				Expect(db.NewBeingWatchedBuildEventChannelMap().BeingWatched(channel1)).To(BeTrue())
				Expect(db.NewBeingWatchedBuildEventChannelMap().BeingWatched(channel2)).To(BeTrue())
			})

			Context("when a build is completed", func() {
				BeforeEach(func() {
					err := build1.Finish(db.BuildStatusAborted)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should delete build1", func() {
					Expect(db.NewBeingWatchedBuildEventChannelMap().BeingWatched(channel1)).To(BeFalse())
					Expect(db.NewBeingWatchedBuildEventChannelMap().BeingWatched(channel2)).To(BeTrue())
				})
			})
		})
	})
})
