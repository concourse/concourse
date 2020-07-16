package creds_test

import (
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/vars"

	// load dummy credential manager
	_ "github.com/concourse/concourse/atc/creds/dummy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Context("pool", func() {
	var (
		logger           lager.Logger
		factory          creds.ManagerFactory
		varSourcePool    creds.VarSourcePool
		config1, config2 map[string]interface{}
		fakeClock        *fakeclock.FakeClock
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("pool-test")
		factory = creds.ManagerFactories()["dummy"]

		config1 = map[string]interface{}{
			"vars": map[string]interface{}{"k1": "v1"},
		}

		config2 = map[string]interface{}{
			"vars": map[string]interface{}{"k2": "v2"},
		}

		fakeClock = fakeclock.NewFakeClock(time.Now())
	})

	Context("FindOrCreate", func() {
		BeforeEach(func() {
			varSourcePool = creds.NewVarSourcePool(logger, 5*time.Minute, time.Minute, fakeClock)
		})

		AfterEach(func() {
			varSourcePool.Close()
		})

		Context("add 1 config", func() {
			var (
				secrets creds.Secrets
				err     error
			)

			JustBeforeEach(func() {
				secrets, err = varSourcePool.FindOrCreate(logger, config1, factory)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should get k1", func() {
				v, _, found, err := secrets.Get(vars.VariableReference{Path: "k1"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(v.(string)).To(Equal("v1"))
			})

			It("should not get foo", func() {
				_, _, found, err := secrets.Get(vars.VariableReference{Path: "foo"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})

			It("pool size should be 1", func() {
				Expect(varSourcePool.Size()).To(Equal(1))
			})
		})

		Context("add 2 configs", func() {
			var (
				secrets1, secrets2 creds.Secrets
				err                error
			)
			JustBeforeEach(func() {
				secrets1, err = varSourcePool.FindOrCreate(logger, config1, factory)
				Expect(err).ToNot(HaveOccurred())
				secrets2, err = varSourcePool.FindOrCreate(logger, config2, factory)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should get k1", func() {
				v, _, found, err := secrets1.Get(vars.VariableReference{Path: "k1"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(v.(string)).To(Equal("v1"))
			})

			It("should get k2", func() {
				v, _, found, err := secrets2.Get(vars.VariableReference{Path: "k2"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(v.(string)).To(Equal("v2"))
			})

			It("should not get foo", func() {
				_, _, found, err := secrets1.Get(vars.VariableReference{Path: "foo"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())

				_, _, found, err = secrets2.Get(vars.VariableReference{Path: "foo"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})

			It("pool size should be 2", func() {
				Expect(varSourcePool.Size()).To(Equal(2))
			})
		})

		Context("add same config for multiple times", func() {
			var (
				secrets1, secrets2 creds.Secrets
				err                error
			)
			JustBeforeEach(func() {
				secrets1, err = varSourcePool.FindOrCreate(logger, config1, factory)
				Expect(err).ToNot(HaveOccurred())
				secrets1, err = varSourcePool.FindOrCreate(logger, config1, factory)
				Expect(err).ToNot(HaveOccurred())
				secrets1, err = varSourcePool.FindOrCreate(logger, config1, factory)
				Expect(err).ToNot(HaveOccurred())
				secrets2, err = varSourcePool.FindOrCreate(logger, config2, factory)
				Expect(err).ToNot(HaveOccurred())
				secrets2, err = varSourcePool.FindOrCreate(logger, config2, factory)
				Expect(err).ToNot(HaveOccurred())
				secrets2, err = varSourcePool.FindOrCreate(logger, config2, factory)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should get k1", func() {
				v, _, found, err := secrets1.Get(vars.VariableReference{Path: "k1"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(v.(string)).To(Equal("v1"))
			})

			It("should get k2", func() {
				v, _, found, err := secrets2.Get(vars.VariableReference{Path: "k2"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(v.(string)).To(Equal("v2"))
			})

			It("should not get foo", func() {
				_, _, found, err := secrets1.Get(vars.VariableReference{Path: "foo"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())

				_, _, found, err = secrets2.Get(vars.VariableReference{Path: "foo"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})

			It("pool size should be 2", func() {
				Expect(varSourcePool.Size()).To(Equal(2))
			})
		})
	})

	Describe("Close", func() {
		var err error

		BeforeEach(func() {
			varSourcePool = creds.NewVarSourcePool(logger, 7*time.Second, 1*time.Second, fakeClock)
		})

		It("cleans up all var sources", func() {
			_, err = varSourcePool.FindOrCreate(logger, config1, factory)
			Expect(err).ToNot(HaveOccurred())
			Expect(varSourcePool.Size()).To(Equal(1))

			fakeClock.WaitForWatcherAndIncrement(4 * time.Second)
			_, err = varSourcePool.FindOrCreate(logger, config2, factory)
			Expect(err).ToNot(HaveOccurred())
			Expect(varSourcePool.Size()).To(Equal(2))

			varSourcePool.Close()
			Eventually(varSourcePool.Size).Should(Equal(0))
		})
	})

	Describe("Garbage Collection", func() {
		var err error

		BeforeEach(func() {
			varSourcePool = creds.NewVarSourcePool(logger, 7*time.Second, 1*time.Second, fakeClock)
		})

		AfterEach(func() {
			varSourcePool.Close()
		})

		It("should clean up once ttl expires", func() {
			_, err = varSourcePool.FindOrCreate(logger, config1, factory)
			Expect(err).ToNot(HaveOccurred())
			Expect(varSourcePool.Size()).To(Equal(1))

			fakeClock.WaitForWatcherAndIncrement(4 * time.Second)
			_, err = varSourcePool.FindOrCreate(logger, config2, factory)
			Expect(err).ToNot(HaveOccurred())
			Expect(varSourcePool.Size()).To(Equal(2))

			fakeClock.WaitForWatcherAndIncrement(4 * time.Second)
			Eventually(varSourcePool.Size).Should(Equal(1))

			fakeClock.WaitForWatcherAndIncrement(4 * time.Second)
			Eventually(varSourcePool.Size).Should(Equal(0))
		})
	})
})
