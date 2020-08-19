package flaghelpers_test

import (
	. "github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InstanceVarPairFlag", func() {
	var flag *InstanceVarPairFlag

	BeforeEach(func() {
		flag = &InstanceVarPairFlag{}
	})

	Describe("UnmarshalFlag", func() {
		Context("when the unmarshalling the key", func() {
			It("unmarshal the flag successfully", func() {
				err := flag.UnmarshalFlag("branch=master")
				Expect(err).ToNot(HaveOccurred())
				Expect(flag.Name).To(Equal("branch"))
			})
		})

		Context("when the unmarshalling the value", func() {
			Context("and the value is a string", func() {
				It("unmarshal the flag successfully", func() {
					err := flag.UnmarshalFlag("branch=master")
					Expect(err).ToNot(HaveOccurred())
					Expect(flag.Value).To(Equal("master"))
				})
			})

			Context("and the value is numeric", func() {
				Context("and represented as numeric", func() {
					It("unmarshal the flag successfully", func() {
						err := flag.UnmarshalFlag("version=1")
						Expect(err).ToNot(HaveOccurred())
						Expect(flag.Value).To(Equal(1))
					})
				})

				Context("and represented as string", func() {
					It("unmarshal the flag successfully", func() {
						err := flag.UnmarshalFlag("version=\"1\"")
						Expect(err).ToNot(HaveOccurred())
						Expect(flag.Value).To(Equal("1"))
					})
				})
			})

			Context("and the value is a nested json", func() {
				It("unmarshal the flag successfully", func() {
					err := flag.UnmarshalFlag("foo={\"bar\":\"baz\",\"version\":1}")
					Expect(err).ToNot(HaveOccurred())
					Expect(flag.Value).To(Equal(map[interface{}]interface{}{"bar": "baz", "version": 1}))
				})
			})
		})
	})
})
