package db_test

import (
	"github.com/concourse/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TaskCacheFactory", func() {

	Describe("FindOrCreate", func() {
		Context("when there is no existing task cache", func() {
			It("creates resource cache in database", func() {
				usedTaskCache, err := taskCacheFactory.FindOrCreate(
					defaultJob.ID(),
					"some-step",
					"some-path",
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(usedTaskCache.ID()).ToNot(BeNil())
			})
		})

		Context("when there is existing task cache", func() {
			var (
				usedTaskCache db.UsedTaskCache
				err           error
			)

			BeforeEach(func() {
				usedTaskCache, err = taskCacheFactory.FindOrCreate(
					defaultJob.ID(),
					"some-step",
					"some-path",
				)
				Expect(err).ToNot(HaveOccurred())
			})

			It("creates a new task cache for another task", func() {
				otherTaskCache, err := taskCacheFactory.FindOrCreate(
					defaultJob.ID(),
					"some-other-step",
					"some-path",
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(otherTaskCache.ID()).ToNot(Equal(usedTaskCache.ID()))
			})
		})
	})

	Describe("Find", func() {
		Context("when there is no existing task cache", func() {
			It("returns no found", func() {
				usedTaskCache, found, err := taskCacheFactory.Find(
					defaultJob.ID(),
					"some-step",
					"some-path",
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(usedTaskCache).To(BeNil())
			})
		})

		Context("when there is existing task cache", func() {
			var (
				usedTaskCache db.UsedTaskCache
				err           error
			)

			BeforeEach(func() {
				usedTaskCache, err = taskCacheFactory.FindOrCreate(
					defaultJob.ID(),
					"some-step",
					"some-path",
				)
				Expect(err).ToNot(HaveOccurred())
			})

			It("finds task cache in database", func() {
				utc, found, err := taskCacheFactory.Find(defaultJob.ID(), "some-step", "some-path")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(utc.ID()).To(Equal(usedTaskCache.ID()))
			})
		})
	})
})
