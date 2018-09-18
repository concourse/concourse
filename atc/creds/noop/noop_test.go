package noop_test

import (
	"github.com/cloudfoundry/bosh-cli/director/template"
	. "github.com/concourse/atc/creds/noop"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Noop", func() {
	var noop Noop

	BeforeEach(func() {
		noop = Noop{}
	})

	Describe("Get", func() {
		var val interface{}
		var found bool
		var getErr error

		JustBeforeEach(func() {
			val, found, getErr = noop.Get(template.VariableDefinition{})
		})

		It("never locates the variable", func() {
			Expect(val).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(getErr).ToNot(HaveOccurred())
		})
	})
})
