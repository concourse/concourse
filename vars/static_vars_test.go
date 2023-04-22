package vars_test

import (
	. "github.com/onsi/ginkgo/v2"
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

		It("returns nil and not found if source is from local vars", func() {
			a := StaticVariables{"a": "foo"}

			val, found, err := a.Get(Reference{Source: ".", Path: "a"})
			Expect(val).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns nil and not found if source is from var source", func() {
			a := StaticVariables{"a": "foo"}

			val, found, err := a.Get(Reference{Source: "some-var-source", Path: "a"})
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
					{Ref: Reference{Path: "hello"}, Value: "world"},
					{Ref: Reference{Path: "foo", Fields: []string{"bar"}}, Value: "baz"},
					{Ref: Reference{Path: "foo", Fields: []string{"abc", "def"}}, Value: "ghi"},
					{Ref: Reference{Path: "foo", Fields: []string{"abc", "jkl"}}, Value: "mno"},
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
					{Ref: Reference{Path: "hello"}, Value: "world"},
					{Ref: Reference{Path: "foo"}, Value: map[string]interface{}{"bar": "baz"}},
				},
				expanded: StaticVariables{
					"hello": "world",
					"foo":   map[string]interface{}{"bar": "baz"},
				},
			},
			{
				desc: "merges recurses through fields",
				kvPairs: KVPairs{
					{Ref: Reference{Path: "hello", Fields: []string{"a", "b"}}, Value: "world"},
					{Ref: Reference{Path: "foo"}, Value: map[string]interface{}{"bar": map[string]interface{}{"abc": "def"}}},
					{Ref: Reference{Path: "foo", Fields: []string{"bar", "ghi"}}, Value: "jkl"},
				},
				expanded: StaticVariables{
					"hello": map[string]interface{}{"a": map[string]interface{}{"b": "world"}},
					"foo":   map[string]interface{}{"bar": map[string]interface{}{"abc": "def", "ghi": "jkl"}},
				},
			},
			{
				desc: "overwrites non-map nodes",
				kvPairs: KVPairs{
					{Ref: Reference{Path: "foo"}, Value: map[string]interface{}{"bar": "baz"}},
					{Ref: Reference{Path: "foo", Fields: []string{"bar", "ghi"}}, Value: "jkl"},
				},
				expanded: StaticVariables{
					"foo": map[string]interface{}{"bar": map[string]interface{}{"ghi": "jkl"}},
				},
			},
			{
				desc: "overwrites full nodes",
				kvPairs: KVPairs{
					{Ref: Reference{Path: "foo"}, Value: map[string]interface{}{"bar": "baz"}},
					{Ref: Reference{Path: "foo"}, Value: "jkl"},
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
