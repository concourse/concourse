package opa_test

import (
	"github.com/concourse/concourse/atc/policy/opa"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OPA Result", func() {
	Context("ParseOpaResult", func() {
		Context("when result string doesn't contain the key of allowed", func() {
			It("should fail", func() {
				_, err := opa.ParseOpaResult([]byte(`{"some": "value"}`), opa.OpaConfig{
					ResultAllowedKey: "a.b",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("allowed: key 'a.b' not found"))
			})
		})

		Context("when result string contain the key of allowed but type is not bool", func() {
			It("should fail", func() {
				_, err := opa.ParseOpaResult([]byte(`{"a": {"b": "ok"}}`), opa.OpaConfig{
					ResultAllowedKey: "a.b",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("allowed: key 'a.b' must have a boolean value"))
			})
		})

		Context("when result string contain the key of allowed with too few parts", func() {
			It("should fail", func() {
				_, err := opa.ParseOpaResult([]byte(`{"a": {"b": true}}`), opa.OpaConfig{
					ResultAllowedKey: "a",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("allowed: key 'a' must have a boolean value"))
			})
		})

		Context("when result string contain the key of allowed with too much parts", func() {
			It("should fail", func() {
				_, err := opa.ParseOpaResult([]byte(`{"a": {"b": true}}`), opa.OpaConfig{
					ResultAllowedKey: "a.b.c",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("allowed: cannot access field 'c' of non-map value ('bool') from var: a.b.c"))
			})
		})

		Context("when result string contain the key of allowed but other keys", func() {
			It("should succeed with shouldBlock being false", func() {
				result, err := opa.ParseOpaResult([]byte(`{"a": {"b": true}}`), opa.OpaConfig{
					ResultAllowedKey: "a.b",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Allowed()).To(BeTrue())
				Expect(result.ShouldBlock()).To((BeFalse()))
				Expect(result.Messages()).To(BeEmpty())
			})
		})

		Context("when result string contain the key of allowed with false value", func() {
			It("should succeed with shouldBlock being true", func() {
				result, err := opa.ParseOpaResult([]byte(`{"a": {"b": false}}`), opa.OpaConfig{
					ResultAllowedKey: "a.b",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Allowed()).To(BeFalse())
				Expect(result.ShouldBlock()).To((BeTrue()))
				Expect(result.Messages()).To(BeEmpty())
			})
		})

		Context("when result string contain the key of allowed and shouldBlock", func() {
			It("should succeed", func() {
				result, err := opa.ParseOpaResult([]byte(`{"a": {"b": true, "c": true}}`), opa.OpaConfig{
					ResultAllowedKey:     "a.b",
					ResultShouldBlockKey: "a.c",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Allowed()).To(BeTrue())
				Expect(result.ShouldBlock()).To((BeTrue()))
				Expect(result.Messages()).To(BeEmpty())
			})
		})

		Context("when result string contain all keys", func() {
			It("should succeed", func() {
				result, err := opa.ParseOpaResult([]byte(`{"a": {"b": true, "c": true, "d": ["e", "f"]}}`), opa.OpaConfig{
					ResultAllowedKey:     "a.b",
					ResultShouldBlockKey: "a.c",
					ResultMessagesKey:    "a.d",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Allowed()).To(BeTrue())
				Expect(result.ShouldBlock()).To((BeTrue()))
				Expect(result.Messages()).To(Equal([]string{"e", "f"}))
			})
		})
	})
})
