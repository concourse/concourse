package creds_test

import (
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NamedSecrets", func() {
	var (
		fakeSecrets      *credsfakes.FakeSecrets
		secretLookupPath creds.SecretLookupPath
	)

	BeforeEach(func() {
		secretLookupPath = creds.NewSecretLookupWithPrefix("test")
		fakeSecrets = new(credsfakes.FakeSecrets)
		fakeSecrets.GetReturns("ok", nil, true, nil)
		fakeSecrets.NewSecretLookupPathsReturns([]creds.SecretLookupPath{secretLookupPath})
	})

	Context("With non-empty name", func() {
		var namedSecrets creds.Secrets

		BeforeEach(func() {
			namedSecrets = creds.NewNamedSecrets(fakeSecrets, "myname")
		})

		It("should find var with myname", func() {
			value, _, found, err := namedSecrets.Get("myname:var")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(value.(string)).To(Equal("ok"))
			arg := fakeSecrets.GetArgsForCall(0)
			Expect(arg).To(Equal("var"))
		})

		It("should extra secretPath", func() {
			value, _, found, err := namedSecrets.Get("concourse/some-team/some-pipeline/myname:var")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(value.(string)).To(Equal("ok"))
			arg := fakeSecrets.GetArgsForCall(0)
			Expect(arg).To(Equal("concourse/some-team/some-pipeline/var"))
		})

		It("should not find var with other name", func() {
			_, _, found, err := namedSecrets.Get("foo:var")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("should not find var without name", func() {
			_, _, found, err := namedSecrets.Get("var")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
			Expect(fakeSecrets.GetCallCount()).To(Equal(0))
		})

		It("should raise error for invalid var", func() {
			_, _, found, err := namedSecrets.Get("foo:bar:var")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid var: foo:bar:var"))
			Expect(found).To(BeFalse())
			Expect(fakeSecrets.GetCallCount()).To(Equal(0))
		})
	})

	Context("With empty name", func() {
		var namedSecrets creds.Secrets

		BeforeEach(func() {
			namedSecrets = creds.NewNamedSecrets(fakeSecrets, "")
		})

		It("should find var without name", func() {
			value, _, found, err := namedSecrets.Get("var")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(value.(string)).To(Equal("ok"))
		})

		It("should find var with a name", func() {
			_, _, found, err := namedSecrets.Get("foo:var")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("should raise error for invalid var", func() {
			_, _, found, err := namedSecrets.Get("foo:bar:var")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid var: foo:bar:var"))
			Expect(found).To(BeFalse())
		})
	})
})
