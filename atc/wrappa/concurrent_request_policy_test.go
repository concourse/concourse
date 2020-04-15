package wrappa_test

import (
	"sync"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/wrappa"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent Request Policy", func() {
	Describe("ConcurrentRequestLimitFlag#UnmarshalFlag", func() {
		var crl wrappa.ConcurrentRequestLimitFlag

		BeforeEach(func() {
			crl = wrappa.ConcurrentRequestLimitFlag{}
		})

		It("parses the API action and limit", func() {
			crl.UnmarshalFlag("ListAllJobs=3")
			Expect(crl.Action).To(Equal("ListAllJobs"), "wrong action")
			Expect(crl.Limit).To(Equal(3), "wrong limit")
		})

		It("returns an error when the flag has no equals sign", func() {
			err := crl.UnmarshalFlag("banana")
			expected := "invalid concurrent request limit " +
				"'banana': value must be an assignment"
			Expect(err).To(MatchError(expected))
		})

		It("returns an error when the flag has multiple equals signs", func() {
			err := crl.UnmarshalFlag("foo=bar=baz")
			expected := "invalid concurrent request limit " +
				"'foo=bar=baz': value must be an assignment"
			Expect(err).To(MatchError(expected))
		})

		It("returns an error when the action is invalid", func() {
			err := crl.UnmarshalFlag("InvalidAction=0")
			expected := "invalid concurrent request limit " +
				"'InvalidAction=0': " +
				"'InvalidAction' is not a valid action"
			Expect(err).To(MatchError(expected))
		})

		It("returns an error when the limit is not an integer", func() {
			err := crl.UnmarshalFlag("ListAllJobs=foo")
			expected := "invalid concurrent request limit " +
				"'ListAllJobs=foo': limit must be " +
				"a non-negative integer"
			Expect(err).To(MatchError(expected))
		})

		It("returns an error when the limit is negative", func() {
			err := crl.UnmarshalFlag("ListAllJobs=-1")
			expected := "invalid concurrent request limit " +
				"'ListAllJobs=-1': limit must be " +
				"a non-negative integer"
			Expect(err).To(MatchError(expected))
		})
	})

	Describe("ConcurrentRequestPolicy#Validate", func() {
		It("raises an error when the action is not supported", func() {
			policy := wrappa.NewConcurrentRequestPolicy(
				[]wrappa.ConcurrentRequestLimitFlag{
					wrappa.ConcurrentRequestLimitFlag{
						Action: atc.ListAllJobs,
						Limit:  0,
					},
				},
				[]string{atc.CreateJobBuild},
			)

			err := policy.Validate()

			Expect(err).To(MatchError(
				"invalid concurrent request limit " +
					"'ListAllJobs=0': " +
					"'ListAllJobs' is not supported",
			))
		})

		It("raises an error with multiple limits on the same action", func() {
			policy := wrappa.NewConcurrentRequestPolicy(
				[]wrappa.ConcurrentRequestLimitFlag{
					wrappa.ConcurrentRequestLimitFlag{
						Action: atc.CreateJobBuild,
						Limit:  0,
					},
					wrappa.ConcurrentRequestLimitFlag{
						Action: atc.CreateJobBuild,
						Limit:  0,
					},
				},
				[]string{atc.CreateJobBuild},
			)

			err := policy.Validate()

			Expect(err).To(MatchError(
				"invalid concurrent request limits: " +
					"multiple limits on 'CreateJobBuild'",
			))
		})
	})

	Describe("ConcurrentRequestPolicy#IsLimited", func() {
		It("tells when an action is limited", func() {
			policy := wrappa.NewConcurrentRequestPolicy(
				[]wrappa.ConcurrentRequestLimitFlag{
					wrappa.ConcurrentRequestLimitFlag{
						Action: atc.CreateJobBuild,
						Limit:  0,
					},
				},
				[]string{atc.CreateJobBuild},
			)

			Expect(policy.IsLimited(atc.CreateJobBuild)).To(BeTrue())
		})

		It("tells when an action is not limited", func() {
			policy := wrappa.NewConcurrentRequestPolicy(
				[]wrappa.ConcurrentRequestLimitFlag{
					wrappa.ConcurrentRequestLimitFlag{
						Action: atc.CreateJobBuild,
						Limit:  0,
					},
				},
				[]string{atc.CreateJobBuild},
			)

			Expect(policy.IsLimited(atc.ListAllPipelines)).To(BeFalse())
		})
	})

	Describe("ConcurrentRequestPolicy#HandlerPool", func() {
		It("can acquire a handler", func() {
			pool := pool(1)

			Expect(pool.TryAcquire()).To(BeTrue())
		})

		It("fails to acquire a handler when the limit is reached", func() {
			pool := pool(1)

			pool.TryAcquire()
			Expect(pool.TryAcquire()).To(BeFalse())
		})

		It("can acquire a handler after releasing", func() {
			pool := pool(1)

			pool.TryAcquire()
			pool.Release()
			Expect(pool.TryAcquire()).To(BeTrue())
		})

		It("can acquire multiple handlers", func() {
			pool := pool(2)

			pool.TryAcquire()
			Expect(pool.TryAcquire()).To(BeTrue())
		})

		It("cannot release more handlers than are held", func() {
			pool := pool(1)

			Expect(pool.Release()).NotTo(Succeed())
		})

		It("cannot acquire multiple handlers simultaneously", func() {
			pool := pool(100)
			failed := false

			doInParallel(101, func() {
				if !pool.TryAcquire() {
					failed = true
				}
			})

			Expect(failed).To(BeTrue())
		})

		It("cannot release multiple handlers simultaneously", func() {
			pool := pool(1000)
			doInParallel(1000, func() { pool.TryAcquire() })
			failed := false

			doInParallel(1001, func() {
				if pool.Release() != nil {
					failed = true
				}
			})

			Expect(failed).To(BeTrue())
		})
	})
})

func doInParallel(numGoroutines int, thingToDo func()) {
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			thingToDo()
			wg.Done()
		}()
	}
	wg.Wait()
}

func pool(size int) wrappa.Pool {
	return wrappa.NewConcurrentRequestPolicy(
		[]wrappa.ConcurrentRequestLimitFlag{
			wrappa.ConcurrentRequestLimitFlag{
				Action: atc.CreateJobBuild,
				Limit:  size,
			},
		},
		[]string{atc.CreateJobBuild},
	).HandlerPool(atc.CreateJobBuild)
}
