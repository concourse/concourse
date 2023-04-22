package volume

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volume Promise List", func() {
	var (
		list PromiseList
	)

	BeforeEach(func() {
		list = NewPromiseList()
	})

	Context("promise doesn't exist yet", func() {
		It("can add promise", func() {
			promise := NewPromise()

			err := list.AddPromise("some-handle", promise)

			Expect(err).NotTo(HaveOccurred())
			Expect(list.GetPromise("some-handle")).To(Equal(promise))
		})
	})

	Context("promise already exists", func() {
		It("can't add promise again", func() {
			err := list.AddPromise("some-handle", NewPromise())

			Expect(err).NotTo(HaveOccurred())

			err = list.AddPromise("some-handle", NewPromise())

			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(ErrPromiseAlreadyExists))
		})

		It("can remove promise", func() {
			err := list.AddPromise("some-handle", NewPromise())

			Expect(err).NotTo(HaveOccurred())

			list.RemovePromise("some-handle")

			Expect(list.GetPromise("some-handle")).To(BeNil())
		})
	})
})
