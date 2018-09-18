package hijackhelpers_test

import (
	"fmt"
	"sort"

	"github.com/concourse/atc"
	. "github.com/concourse/fly/commands/internal/hijackhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerSorter", func() {
	var containers []atc.Container
	var sorter ContainerSorter

	BeforeEach(func() {
		containers = []atc.Container{
			{
				ID:       "8",
				BuildID:  2,
				StepName: "first",
				Type:     "task",
			},
			{
				ID:       "3",
				BuildID:  1,
				StepName: "first",
				Type:     "check",
			},
			{
				ID:       "6",
				BuildID:  1,
				StepName: "second",
				Type:     "get",
			},
			{
				ID:       "4",
				BuildID:  1,
				StepName: "first",
				Type:     "get",
			},
			{
				ID:       "7",
				BuildID:  2,
				StepName: "first",
				Type:     "put",
			},
			{
				ID:       "5",
				BuildID:  1,
				StepName: "second",
				Type:     "check",
			},
			{
				ID:       "9",
				BuildID:  3,
				StepName: "first",
				Type:     "check",
			},
			{
				ID:           "2",
				ResourceName: "zed",
			},
			{
				ID:           "1",
				ResourceName: "clarity",
			},
		}
		sorter = ContainerSorter(containers)
	})

	Context("implements Sort interface", func() {
		Context("Len", func() {
			It("is the number of elements in the collection", func() {
				Expect(sorter.Len()).To(Equal(len(containers)))
			})
		})

		Context("Swap", func() {
			It("swaps the elements with indexes i and j", func() {
				sorter.Swap(0, 1)
				Expect(sorter[0].ID).To(Equal("3"))
				Expect(sorter[1].ID).To(Equal("8"))
			})
		})

		Context("Less", func() {
			Context("reports whether the element with index i should sort before the element with index j", func() {
				Context("when BuildID of i is less than j", func() {
					It("returns true", func() {
						Expect(sorter.Less(1, 6)).To(BeTrue())
					})
				})

				Context("when BuildID of i is greater than j", func() {
					It("returns false", func() {
						Expect(sorter.Less(0, 1)).To(BeFalse())
					})
				})

				Context("when BuildID of i is equal to j", func() {
					Context("handling background check containers", func() {
						Context("when ResourceName of i is less than j", func() {
							It("returns true", func() {
								Expect(sorter.Less(8, 7)).To(BeTrue())
							})
						})

						Context("when ResourceName of i is greater than j", func() {
							It("returns false", func() {
								Expect(sorter.Less(7, 8)).To(BeFalse())
							})
						})
					})

					Context("when StepName of i is less than j", func() {
						It("returns true", func() {
							Expect(sorter.Less(1, 2)).To(BeTrue())
						})
					})

					Context("when StepName of i is greater than j", func() {
						It("returns false", func() {
							Expect(sorter.Less(2, 1)).To(BeFalse())
						})
					})

					Context("when SetName of i is equal to j", func() {
						Context("when Type of i is less than j", func() {
							It("returns true", func() {
								Expect(sorter.Less(1, 3)).To(BeTrue())
							})
						})

						Context("when Type of i is greater than j", func() {
							It("returns false", func() {
								Expect(sorter.Less(3, 1)).To(BeFalse())
							})
						})

						Context("when Type of i is equal to j", func() {
							It("returns false", func() {
								Expect(sorter.Less(1, 1)).To(BeFalse())
							})
						})
					})
				})
			})
		})
	})

	Context("Sort", func() {
		It("returns a sorted list of containers", func() {
			sort.Sort(sorter)

			for i, container := range sorter {
				Expect(container.ID).To(Equal(fmt.Sprint(i + 1)))
			}
		})
	})
})
