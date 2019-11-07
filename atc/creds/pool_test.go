package creds_test

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/gc"
	"time"

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
	})

	Context("FindOrCreate", func() {
		BeforeEach(func() {
			varSourcePool = creds.NewVarSourcePool(5 * time.Minute)
		})

		Context("add 1 config", func() {
			var (
				secrets creds.Secrets
				err error
			)

			JustBeforeEach(func() {
				secrets, err = varSourcePool.FindOrCreate(logger, config1, factory)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should get k1", func() {
				v, _, found, err := secrets.Get("k1")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(v.(string)).To(Equal("v1"))
			})

			It("should not get foo", func() {
				_, _, found, err := secrets.Get("foo")
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
				err error
			)
			JustBeforeEach(func() {
				secrets1, err = varSourcePool.FindOrCreate(logger, config1, factory)
				Expect(err).ToNot(HaveOccurred())
				secrets2, err = varSourcePool.FindOrCreate(logger, config2, factory)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should get k1", func() {
				v, _, found, err := secrets1.Get("k1")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(v.(string)).To(Equal("v1"))
			})

			It("should get k2", func() {
				v, _, found, err := secrets2.Get("k2")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(v.(string)).To(Equal("v2"))
			})

			It("should not get foo", func() {
				_, _, found, err := secrets1.Get("foo")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())

				_, _, found, err = secrets2.Get("foo")
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
				err error
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
				v, _, found, err := secrets1.Get("k1")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(v.(string)).To(Equal("v1"))
			})

			It("should get k2", func() {
				v, _, found, err := secrets2.Get("k2")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(v.(string)).To(Equal("v2"))
			})

			It("should not get foo", func() {
				_, _, found, err := secrets1.Get("foo")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())

				_, _, found, err = secrets2.Get("foo")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})

			It("pool size should be 2", func() {
				Expect(varSourcePool.Size()).To(Equal(2))
			})
		})
	})

	Context("Collect", func() {
		var err error

		BeforeEach(func() {
			varSourcePool = creds.NewVarSourcePool(4 * time.Second)
		})
		It("should clean up once ttl expires", func() {
			_, err = varSourcePool.FindOrCreate(logger, config1, factory)
			Expect(err).ToNot(HaveOccurred())
			Expect(varSourcePool.Size()).To(Equal(1))

			time.Sleep(2*time.Second)
			_, err = varSourcePool.FindOrCreate(logger, config2, factory)
			Expect(err).ToNot(HaveOccurred())
			Expect(varSourcePool.Size()).To(Equal(2))

			time.Sleep(2*time.Second)
			err = varSourcePool.(gc.Collector).Collect(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(varSourcePool.Size()).To(Equal(1))

			time.Sleep(2*time.Second)
			err = varSourcePool.(gc.Collector).Collect(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(varSourcePool.Size()).To(Equal(0))
		})
	})
})
