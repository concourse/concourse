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

			val, found, err := a.Get(VariableDefinition{Ref: VariableReference{Path: "a"}})
			Expect(val).To(Equal("foo"))
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns nil and not found if key is not found", func() {
			a := StaticVariables{"a": "foo"}

			val, found, err := a.Get(VariableDefinition{Ref: VariableReference{Path: "b"}})
			Expect(val).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
		})

		It("recognizes keys that use dot notation for subvalues", func() {
			a := StaticVariables{"a.subkey": "foo", "a.subkey2": "foo2"}

			val, found, err := a.Get(VariableDefinition{Ref: VariableReference{Path: "a.subkey"}})
			Expect(val).To(Equal("foo"))
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("List", func() {
		It("returns list of names", func() {
			defs, err := StaticVariables{}.List()
			Expect(defs).To(BeEmpty())
			Expect(err).ToNot(HaveOccurred())

			defs, err = StaticVariables{"a": "1", "b": "2"}.List()
			Expect(defs).To(ConsistOf([]VariableDefinition{
				{Ref: VariableReference{Path: "a"}},
				{Ref: VariableReference{Path: "b"}},
			}))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Flatten", func() {
		for _, tt := range []struct {
			desc     string
			expanded StaticVariables
			kvPairs  KVPairs
		}{
			{
				desc: "flattens recursively",
				expanded: StaticVariables{
					"hello": "world",
					"foo":   map[string]interface{}{"bar": "baz", "abc": map[interface{}]interface{}{"def": "ghi", "jkl": "mno"}},
				},
				kvPairs: KVPairs{
					{Ref: VariableReference{Path: "hello"}, Value: "world"},
					{Ref: VariableReference{Path: "foo", Fields: []string{"bar"}}, Value: "baz"},
					{Ref: VariableReference{Path: "foo", Fields: []string{"abc", "def"}}, Value: "ghi"},
					{Ref: VariableReference{Path: "foo", Fields: []string{"abc", "jkl"}}, Value: "mno"},
				},
			},
		} {
			tt := tt
			It(tt.desc, func() {
				Expect(tt.expanded.Flatten()).To(ConsistOf(tt.kvPairs))
			})
		}
	})

	Describe("Expand", func() {
		for _, tt := range []struct {
			desc     string
			kvPairs  KVPairs
			expanded StaticVariables
		}{
			{
				desc: "merges flat elements into map",
				kvPairs: KVPairs{
					{Ref: VariableReference{Path: "hello"}, Value: "world"},
					{Ref: VariableReference{Path: "foo"}, Value: map[string]interface{}{"bar": "baz"}},
				},
				expanded: StaticVariables{
					"hello": "world",
					"foo":   map[string]interface{}{"bar": "baz"},
				},
			},
			{
				desc: "merges recurses through fields",
				kvPairs: KVPairs{
					{Ref: VariableReference{Path: "hello", Fields: []string{"a", "b"}}, Value: "world"},
					{Ref: VariableReference{Path: "foo"}, Value: map[string]interface{}{"bar": map[string]interface{}{"abc": "def"}}},
					{Ref: VariableReference{Path: "foo", Fields: []string{"bar", "ghi"}}, Value: "jkl"},
				},
				expanded: StaticVariables{
					"hello": map[string]interface{}{"a": map[string]interface{}{"b": "world"}},
					"foo":   map[string]interface{}{"bar": map[string]interface{}{"abc": "def", "ghi": "jkl"}},
				},
			},
			{
				desc: "overwrites non-map nodes",
				kvPairs: KVPairs{
					{Ref: VariableReference{Path: "foo"}, Value: map[string]interface{}{"bar": "baz"}},
					{Ref: VariableReference{Path: "foo", Fields: []string{"bar", "ghi"}}, Value: "jkl"},
				},
				expanded: StaticVariables{
					"foo": map[string]interface{}{"bar": map[string]interface{}{"ghi": "jkl"}},
				},
			},
			{
				desc: "overwrites full nodes",
				kvPairs: KVPairs{
					{Ref: VariableReference{Path: "foo"}, Value: map[string]interface{}{"bar": "baz"}},
					{Ref: VariableReference{Path: "foo"}, Value: "jkl"},
				},
				expanded: StaticVariables{
					"foo": "jkl",
				},
			},
		} {
			tt := tt
			It(tt.desc, func() {
				Expect(tt.kvPairs.Expand()).To(Equal(tt.expanded))
			})
		}
	})
})
