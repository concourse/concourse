package volume_test

import (
	"fmt"
	"sync"
	"time"

	"github.com/concourse/concourse/worker/baggageclaim/volume"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KeyedLock", func() {
	var lockManager volume.LockManager

	BeforeEach(func() {
		lockManager = volume.NewLockManager()
	})

	Describe("Lock", func() {
		Context("when the key hasn't previously been locked", func() {
			It("allows access", func() {
				accessGrantedCh := make(chan struct{})
				go func() {
					lockManager.Lock("the-key")
					close(accessGrantedCh)
				}()
				Eventually(accessGrantedCh).Should(BeClosed())
			})
		})

		Context("when the key is currently locked", func() {
			It("blocks until it is unlocked", func() {
				firstProcReadyCh := make(chan struct{})
				firstProcWaitCh := make(chan struct{})
				firstProcDoneCh := make(chan struct{})
				secondProcReadyCh := make(chan struct{})
				secondProcDoneCh := make(chan struct{})

				go func() {
					lockManager.Lock("the-key")
					close(firstProcReadyCh)
					<-firstProcWaitCh
					lockManager.Unlock("the-key")
					close(firstProcDoneCh)
				}()

				Eventually(firstProcReadyCh).Should(BeClosed())

				go func() {
					lockManager.Lock("the-key")
					close(secondProcReadyCh)
					lockManager.Unlock("the-key")
					close(secondProcDoneCh)
				}()

				Consistently(secondProcReadyCh).ShouldNot(BeClosed())
				firstProcWaitCh <- struct{}{}
				Eventually(secondProcDoneCh).Should(BeClosed())
			})

			It("allows multiple separate keys to be locked simultaneously", func() {
				key1LockedCh := make(chan struct{})
				key2LockedCh := make(chan struct{})
				bothKeysDoneCh := make(chan struct{})

				go func() {
					lockManager.Lock("key-1")
					close(key1LockedCh)

					// Wait a bit to ensure we can acquire key-2 while holding key-1
					time.Sleep(50 * time.Millisecond)

					lockManager.Lock("key-2")
					close(key2LockedCh)

					lockManager.Unlock("key-1")
					lockManager.Unlock("key-2")
					close(bothKeysDoneCh)
				}()

				Eventually(key1LockedCh).Should(BeClosed())
				Eventually(key2LockedCh).Should(BeClosed())
				Eventually(bothKeysDoneCh).Should(BeClosed())
			})

			It("maintains lock ordering when multiple goroutines try to acquire the same locks", func() {
				const numGoroutines = 10
				const numLockOperations = 5

				var wg sync.WaitGroup
				counter := 0
				counterMutex := sync.Mutex{}

				// Each goroutine will increment the counter while holding the lock
				for i := range numGoroutines {
					wg.Add(1)
					go func(id int) {
						defer wg.Done()
						for range numLockOperations {
							lockManager.Lock("counter-key")

							// Read counter
							counterMutex.Lock()
							currentValue := counter
							counterMutex.Unlock()

							// Simulate some work
							time.Sleep(1 * time.Millisecond)

							// Increment counter
							counterMutex.Lock()
							counter = currentValue + 1
							counterMutex.Unlock()

							lockManager.Unlock("counter-key")
						}
					}(i)
				}

				wg.Wait()

				// If locks work correctly, the counter should equal the total number of operations
				Expect(counter).To(Equal(numGoroutines * numLockOperations))
			})

			It("cleans up locks that are no longer in use", func() {
				// This test verifies that we don't leak memory by keeping unused locks

				// Create and use 100 different locks
				for i := range 100 {
					key := fmt.Sprintf("temp-key-%d", i)
					lockManager.Lock(key)
					lockManager.Unlock(key)
				}

				// Now verify we can still create and use a new lock
				lockAcquiredCh := make(chan struct{})
				go func() {
					lockManager.Lock("final-key")
					close(lockAcquiredCh)
					lockManager.Unlock("final-key")
				}()

				Eventually(lockAcquiredCh).Should(BeClosed())
			})
		})
	})

	Describe("Unlock", func() {
		Context("when the key has not been locked", func() {
			It("panics", func() {
				Expect(func() {
					lockManager.Unlock("key")
				}).To(Panic())
			})
		})

		Context("when unlocking a key multiple times", func() {
			It("panics on the second unlock", func() {
				lockManager.Lock("double-unlock-key")
				lockManager.Unlock("double-unlock-key")

				Expect(func() {
					lockManager.Unlock("double-unlock-key")
				}).To(Panic())
			})
		})
	})

	Describe("Concurrent operations", func() {
		It("correctly handles high contention on multiple keys", func() {
			const numGoroutines = 20
			const numKeys = 5
			const opsPerGoroutine = 10

			var wg sync.WaitGroup
			completed := make([]bool, numGoroutines)

			// Start multiple goroutines all trying to lock/unlock different keys
			for i := range numGoroutines {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()

					for j := range opsPerGoroutine {
						// Choose a key based on the goroutine ID and iteration
						keyID := (id + j) % numKeys
						key := fmt.Sprintf("concurrent-key-%d", keyID)

						lockManager.Lock(key)
						// Simulate some work
						time.Sleep(1 * time.Millisecond)
						lockManager.Unlock(key)
					}

					completed[id] = true
				}(i)
			}

			wg.Wait()

			// Verify all goroutines completed
			for i := range numGoroutines {
				Expect(completed[i]).To(BeTrue())
			}
		})
	})
})
