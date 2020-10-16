package vars_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/concourse/vars"
)

var _ = Describe("StaticVariables", func() {
	Describe("Get", func() {
		It("returns value and found if key is found", func() {
			a := StaticVariables{"a": "foo"}

			val, found, err := a.Get(Reference{Path: "a"})
			Expect(val).To(Equal("foo"))
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns nil and not found if key is not found", func() {
			a := StaticVariables{"a": "foo"}

			val, found, err := a.Get(Reference{Path: "b"})
			Expect(val).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
		})

		It("follows fields", func() {
			v := StaticVariables{
				"a": map[string]interface{}{
					"subkey1": map[interface{}]interface{}{
						"subkey2": "foo",
					},
				}}

			val, found, err := v.Get(Reference{Path: "a", Fields: []string{"subkey1", "subkey2"}})
			Expect(val).To(Equal("foo"))
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when fields don't exist", func() {
			It("errors with a MissingFieldError error", func() {
				v := StaticVariables{
					"a": map[string]interface{}{
						"subkey1": map[interface{}]interface{}{
							"subkey2": "foo",
						},
					}}

				_, _, err := v.Get(Reference{Path: "a", Fields: []string{"subkey1", "bad_key"}})
				_, ok := err.(MissingFieldError)
				Expect(ok).To(BeTrue(), "unexpected error type %T", err)
			})
		})

		Context("when fields cannot be recursed", func() {
			It("errors with an InvalidFieldError error", func() {
				v := StaticVariables{
					"a": map[string]interface{}{
						"subkey1": map[interface{}]interface{}{
							"subkey2": "foo",
						},
					}}

				_, _, err := v.Get(Reference{Path: "a", Fields: []string{"subkey1", "subkey2", "cant_go_deeper"}})
				_, ok := err.(InvalidFieldError)
				Expect(ok).To(BeTrue(), "unexpected error type %T", err)
			})
		})
	})

	Describe("List", func() {
		It("returns list of names", func() {
			defs, err := StaticVariables{}.List()
			Expect(defs).To(BeEmpty())
			Expect(err).ToNot(HaveOccurred())

			defs, err = StaticVariables{"a": "1", "b": "2"}.List()
			Expect(defs).To(ConsistOf([]Reference{
				{Path: "a"},
				{Path: "b"},
			}))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
