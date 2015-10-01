package pagination_test

import (
	. "github.com/concourse/atc/web/pagination"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Paginator", func() {
	Context("PaginationData", func() {
		Context("HasOlder", func() {
			It("returns true if ResultsGreaterThanStartingID is true", func() {
				paginationData := NewPaginationData(true, false, 0, 0, 0)
				Expect(paginationData.HasOlder()).To(BeTrue())
			})

			It("returns true if MoreResultsInGivenDirection is true", func() {
				paginationData := NewPaginationData(false, true, 0, 0, 0)
				Expect(paginationData.HasOlder()).To(BeTrue())
			})

			It("returns false if ResultsGreaterThanStartingID is false and MoreResultsInGivenDirection is false", func() {
				paginationData := NewPaginationData(false, false, 0, 0, 0)
				Expect(paginationData.HasOlder()).To(BeFalse())
			})
		})

		Context("HasNewer", func() {
			It("returns true if dbMaxID is greater than maxIDFromResults", func() {
				paginationData := NewPaginationData(false, false, 5, 4, 1)
				Expect(paginationData.HasNewer()).To(BeTrue())
			})

			It("returns false if dbMaxID equal to maxIDFromResults", func() {
				paginationData := NewPaginationData(false, false, 5, 5, 1)
				Expect(paginationData.HasNewer()).To(BeFalse())
			})
		})

		Context("HasPagination", func() {
			It("returns true if ResultsGreaterThanStartingID is true", func() {
				paginationData := NewPaginationData(true, false, 0, 0, 0)
				Expect(paginationData.HasPagination()).To(BeTrue())
			})

			It("returns true if MoreResultsInGivenDirection is true", func() {
				paginationData := NewPaginationData(false, true, 0, 0, 0)
				Expect(paginationData.HasPagination()).To(BeTrue())
			})

			It("returns true if dbMaxID is greater than maxIDFromResults", func() {
				paginationData := NewPaginationData(false, false, 5, 4, 1)
				Expect(paginationData.HasPagination()).To(BeTrue())
			})
			It("returns false if ResultsGreaterThanStartingID is false, MoreResultsInGivenDirection is false and dbMaxID equal to tmaxIDFromResults", func() {
				paginationData := NewPaginationData(false, false, 5, 5, 1)
				Expect(paginationData.HasPagination()).To(BeFalse())
			})
		})

		Context("OlderStartID", func() {
			It("returns the min id passed in minus 1", func() {
				paginationData := NewPaginationData(false, false, 5, 5, 3)
				Expect(paginationData.OlderStartID()).To(Equal(2))
			})
		})

		Context("NewerStartID", func() {
			It("returns the max id passed in plus 1", func() {
				paginationData := NewPaginationData(false, false, 7, 5, 3)
				Expect(paginationData.NewerStartID()).To(Equal(6))
			})
		})
	})
})
