package creds_test

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"

	// load dummy credential manager
	_ "github.com/concourse/concourse/atc/creds/dummy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Context("pool", func() {
	var (
		logger lager.Logger
		factory creds.ManagerFactory
		err error
		secrets creds.Secrets
	)

	BeforeEach(func() {
		logger = lager.NewLogger("pool-test")
		factory = creds.ManagerFactories()["dummy"]
	})

	Context("FindOrCreate", func() {
		var (
			config1, config2 map[string]interface{}
		)

		BeforeEach(func() {
			config1 = map[string]interface{}{
				"vars": map[string]interface{}{"k1": "v1"},
			}

			config2 = map[string]interface{}{
				"vars": map[string]interface{}{"k2": "v2"},
			}
		})

		Context("when add config1", func() {
			JustBeforeEach(func() {
				secrets, err = creds.VarSourcePoolInstance().FindOrCreate(logger, config1, factory)
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
		})

		Context("when add config2", func() {
			JustBeforeEach(func() {
				secrets, err = creds.VarSourcePoolInstance().FindOrCreate(logger, config2, factory)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should get k2", func() {
				v, _, found, err := secrets.Get("k2")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(v.(string)).To(Equal("v2"))
			})

			It("should not get foo", func() {
				_, _, found, err := secrets.Get("foo")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
