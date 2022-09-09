package volume_test

import (
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
	})
})
