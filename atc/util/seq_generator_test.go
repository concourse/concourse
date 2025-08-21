package util_test

import (
	"math/rand/v2"
	"sync"
	"time"

	"github.com/concourse/concourse/atc/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sequence Generator", func() {
	It("starting at 0", func() {
		seq := util.NewSequenceGenerator(0)
		next := seq.Next()
		Expect(next).To(Equal(0), "should return 0")
	})

	It("starting at some number", func() {
		n := rand.IntN(101)
		seq := util.NewSequenceGenerator(n)
		Expect(seq.Next()).To(Equal(n), "should return the starting number first")
		Expect(seq.Next()).To(Equal(n+1), "should return the starting number+1 on the second call to Next()")
	})

	It("calling Next() concurrently succeeds", func() {
		seqs := sync.Map{}
		wg := sync.WaitGroup{}
		seq := util.NewSequenceGenerator(0)

		for range 1000 {
			wg.Go(func() {
				defer GinkgoRecover()
				ms := rand.IntN(191) + 10 // Return number between 10-200
				time.Sleep(time.Duration(ms) * time.Millisecond)
				n := seq.Next()
				_, loaded := seqs.LoadOrStore(n, n)
				Expect(loaded).To(BeFalse(), "should never encounter the same value twice")
			})
		}
		wg.Wait()
	})
})
