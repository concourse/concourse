package vars_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/concourse/concourse/vars"
)

var _ = Describe("NamedVariables", func() {
	Describe("Get", func() {
		It("return no value and not found if there are no sources", func() {
			val, found, err := NamedVariables{}.Get(Reference{})
			Expect(val).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
		})

		It("return no value, not found and an error if var source name doesn't exist", func() {
			vars1 := StaticVariables{"key1": "val"}
			vars2 := StaticVariables{"key2": "val"}
			vars := NamedVariables{"s1": vars1, "s2": vars2}

			val, found, err := vars.Get(Reference{Source: "s3", Path: "foo"})
			Expect(val).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("missing source 's3' in var: s3:foo"))
		})

		It("return found value as soon as one source succeeds", func() {
			vars1 := &FakeVariables{}
			vars2 := StaticVariables{"key2": "val"}
			vars3 := &FakeVariables{GetErr: errors.New("fake-err")}
			vars := NamedVariables{"s1": vars1, "s2": vars2, "s3": vars3}

			val, found, err := vars.Get(Reference{Source: "s2", Path: "key2"})
			Expect(val).To(Equal("val"))
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			// Didn't get past other variables
			Expect(vars1.GetCallCount).To(Equal(0))
			Expect(vars3.GetCallCount).To(Equal(0))
		})

		It("return no value and not found if var source name is not specified", func() {
			vars1 := StaticVariables{"key1": "val"}
			vars2 := StaticVariables{"key2": "val"}
			vars := NamedVariables{"s1": vars1, "s2": vars2}

			val, found, err := vars.Get(Reference{Path: "key1"})
			Expect(val).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
		})

		It("return error as soon as one source fails", func() {
			vars1 := StaticVariables{"key1": "val"}
			vars2 := &FakeVariables{GetErr: errors.New("fake-err")}
			vars := NamedVariables{"s1": vars1, "s2": vars2}

			val, found, err := vars.Get(Reference{Source: "s2", Path: "key3"})
			Expect(val).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).To(Equal(errors.New("fake-err")))
		})
	})

	Describe("List", func() {
		It("returns list of names from multiple vars with duplicates", func() {
			defs, err := NamedVariables{}.List()
			Expect(defs).To(BeEmpty())
			Expect(err).ToNot(HaveOccurred())

			vars := NamedVariables{
				"s1": StaticVariables{"a": "1", "b": "2"},
				"s2": StaticVariables{"b": "3", "c": "4"},
			}

			defs, err = vars.List()
			Expect(defs).To(ConsistOf([]Reference{
				{Source: "s1", Path: "a"},
				{Source: "s1", Path: "b"},
				{Source: "s2", Path: "b"},
				{Source: "s2", Path: "c"},
			}))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
