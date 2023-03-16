package util_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc/util"
)

var _ = Describe("Component", func() {
	Describe("ComputeDrift", func() {
		It("should return correct values", func() {
			Expect(util.ComputeDrift(0)).To(Equal(-1 * time.Second))
			Expect(util.ComputeDrift(5000)).To(Equal(-900 * time.Millisecond))
			Expect(util.ComputeDrift(50000)).To(Equal(0 * time.Second))
			Expect(util.ComputeDrift(100000)).To(Equal(time.Second))
			Expect(util.ComputeDrift(200000)).To(Equal(3 * time.Second))
		})
	})
})
