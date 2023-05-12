package db_test

import (
	"github.com/concourse/concourse/atc/db"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BeingWatchedBuildEventChannelMap", func() {
	It("should marked channel be queried", func() {
		m := db.NewBeingWatchedBuildEventChannelMap()

		m.Mark("channel-1")
		Expect(m.BeingWatched("channel-1")).To(BeTrue())
		Expect(m.BeingWatched("channel-2")).To(BeFalse())

		// Condition func always return false, thus no entry should be deleted.
		m.Clean(func(k string, _ time.Time) (string, bool) { return k, false })
		Expect(m.BeingWatched("channel-1")).To(BeTrue())

		// Condition func always return true, thus no entry should be deleted.
		m.Clean(func(k string, _ time.Time) (string, bool) { return k, true })
		Expect(m.BeingWatched("channel-1")).To(BeFalse())
	})
})
