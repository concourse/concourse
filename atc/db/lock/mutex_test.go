package lock_test

import (
	"time"

	"github.com/concourse/concourse/atc/db/lock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Mutex", func() {

	var mutex interface {
		Unlock()
		Lock() bool
		LockWithTimeout(timeout time.Duration) bool
	}

	Context("when a default timeout is provided", func() {

		BeforeEach(func() {
			mutex = lock.NewMutex(time.Millisecond)
		})

		Context("when using the defaults", func() {
			It("times out acquiring the lock", func() {
				ready := make(chan struct{})
				done := make(chan struct{})

				go func() {
					acquired := mutex.Lock()
					Expect(acquired).To(BeTrue())
					close(ready)
				}()

				go func() {
					<-ready
					acquired := mutex.Lock()
					Expect(acquired).To(BeFalse())
					close(done)
				}()

				Eventually(done).Should(BeClosed())
			})
		})

		Context("overriding the defaults", func() {
			It("waits to acquire the lock", func() {
				ready := make(chan struct{})
				done := make(chan struct{})

				go func() {
					acquired := mutex.Lock()
					Expect(acquired).To(BeTrue())
					close(ready)
				}()

				go func() {
					<-ready
					mutex.LockWithTimeout(0)
					close(done)
				}()

				Consistently(done).ShouldNot(Receive())
			})

			It("eventually acquires a lock after its been released", func() {
				ready := make(chan struct{})
				waiting := make(chan struct{})
				unlock := make(chan struct{})
				done := make(chan struct{})

				go func() {
					acquired := mutex.Lock()
					Expect(acquired).To(BeTrue())
					close(ready)

					<-unlock
					mutex.Unlock()
				}()

				go func() {
					<-ready
					acquired := mutex.LockWithTimeout(0)
					close(waiting)

					Expect(acquired).To(BeTrue())
					close(done)
				}()

				Consistently(waiting).ShouldNot(Receive())

				close(unlock)

				Eventually(done).Should(BeClosed())
			})
		})
	})

	Context("when there is no default timeout", func() {

		BeforeEach(func() {
			mutex = lock.NewMutex(0)
		})

		Context("when using the defaults", func() {
			It("waits to acquire the lock", func() {
				ready := make(chan struct{})
				done := make(chan struct{})

				go func() {
					acquired := mutex.Lock()
					Expect(acquired).To(BeTrue())
					close(ready)
				}()

				go func() {
					<-ready
					mutex.Lock()
					close(done)
				}()

				Consistently(done).ShouldNot(Receive())
			})

			It("eventually acquires a lock after its been released", func() {
				ready := make(chan struct{})
				waiting := make(chan struct{})
				unlock := make(chan struct{})
				done := make(chan struct{})

				go func() {
					acquired := mutex.Lock()
					Expect(acquired).To(BeTrue())
					close(ready)

					<-unlock
					mutex.Unlock()
				}()

				go func() {
					<-ready
					acquired := mutex.Lock()
					close(waiting)

					Expect(acquired).To(BeTrue())
					close(done)
				}()

				Consistently(waiting).ShouldNot(Receive())

				close(unlock)

				Eventually(done).Should(BeClosed())
			})
		})

		Context("overriding the defaults", func() {
			It("times out acquiring the lock", func() {
				ready := make(chan struct{})
				done := make(chan struct{})

				go func() {
					acquired := mutex.Lock()
					Expect(acquired).To(BeTrue())
					close(ready)
				}()

				go func() {
					<-ready
					acquired := mutex.LockWithTimeout(time.Millisecond)
					Expect(acquired).To(BeFalse())
					close(done)
				}()

				Eventually(done).Should(BeClosed())
			})
		})
	})
})
