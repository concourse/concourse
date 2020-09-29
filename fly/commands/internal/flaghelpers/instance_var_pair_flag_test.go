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
		Context("when the flag has a single root key", func() {
			Context("and the value is a string", func() {
				It("unmarshal the flag successfully", func() {
					err := flag.UnmarshalFlag("branch=master")
					Expect(err).ToNot(HaveOccurred())
					Expect(flag.Value["branch"]).To(Equal("master"))
				})
			})

			Context("and the value is numeric", func() {
				Context("and represented as numeric", func() {
					It("unmarshal the flag successfully", func() {
						err := flag.UnmarshalFlag("version=1")
						Expect(err).ToNot(HaveOccurred())
						Expect(flag.Value["version"]).To(Equal(1))
					})
				})

				Context("and represented as string", func() {
					It("unmarshal the flag successfully", func() {
						err := flag.UnmarshalFlag("version=\"1\"")
						Expect(err).ToNot(HaveOccurred())
						Expect(flag.Value["version"]).To(Equal("1"))
					})
				})
			})

			Context("and the value is an object", func() {
				It("unmarshal the flag successfully", func() {
					err := flag.UnmarshalFlag("foo.bar=baz,foo.version=1")
					Expect(err).ToNot(HaveOccurred())
					Expect(flag.Value["foo.bar"]).To(Equal("baz"))
					Expect(flag.Value["foo.version"]).To(Equal(1))
				})
			})

			Context("and the value is an array", func() {
				It("unmarshal the flag successfully", func() {
					err := flag.UnmarshalFlag("quz.0=something,quz.1=8,quz.2=false")
					Expect(err).ToNot(HaveOccurred())
					Expect(flag.Value["quz.0"]).To(Equal("something"))
					Expect(flag.Value["quz.1"]).To(Equal(8))
					Expect(flag.Value["quz.2"]).To(Equal(false))
				})
			})
		})

		Context("when the flag has a multiple root keys", func() {
			It("unmarshal the flag successfully", func() {
				err := flag.UnmarshalFlag("foo.bar=baz,foo.version=1,quz.0=something,quz.1=8,quz.2=false")
				Expect(err).ToNot(HaveOccurred())
				Expect(flag.Value["foo.bar"]).To(Equal("baz"))
				Expect(flag.Value["foo.version"]).To(Equal(1))
				Expect(flag.Value["quz.0"]).To(Equal("something"))
				Expect(flag.Value["quz.1"]).To(Equal(8))
				Expect(flag.Value["quz.2"]).To(Equal(false))
			})
		})
	})
})
