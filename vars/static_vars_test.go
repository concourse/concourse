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

			val, found, err := a.Get(VariableDefinition{Name: "a"})
			Expect(val).To(Equal("foo"))
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns nil and not found if key is not found", func() {
			a := StaticVariables{"a": "foo"}

			val, found, err := a.Get(VariableDefinition{Name: "b"})
			Expect(val).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
		})

		It("recognizes keys that use dot notation for subvalues", func() {
			a := StaticVariables{"a.subkey": "foo", "a.subkey2": "foo2"}

			val, found, err := a.Get(VariableDefinition{Name: "a"})
			Expect(val).To(Equal(map[interface{}]interface{}{"subkey": "foo", "subkey2": "foo2"}))
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			a = StaticVariables{"a.subkey.subsubkey": "foo", "a.subkey2": "foo2"}

			val, found, err = a.Get(VariableDefinition{Name: "a"})
			Expect(val).To(Equal(map[interface{}]interface{}{
				"subkey":  map[interface{}]interface{}{"subsubkey": "foo"},
				"subkey2": "foo2",
			}))
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			a = StaticVariables{"a.subkey": "foo"}

			val, found, err = a.Get(VariableDefinition{Name: "a.subkey"})
			Expect(val).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("List", func() {
		It("returns list of names", func() {
			defs, err := StaticVariables{}.List()
			Expect(defs).To(BeEmpty())
			Expect(err).ToNot(HaveOccurred())

			defs, err = StaticVariables{"a": "1", "b": "2"}.List()
			Expect(defs).To(ConsistOf([]VariableDefinition{{Name: "a"}, {Name: "b"}}))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
