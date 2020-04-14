package wrappa_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/wrappa"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent Request Policy", func() {
	Describe("ConcurrentRequestLimitFlag#UnmarshalFlag", func() {
		var crl wrappa.ConcurrentRequestLimitFlag

		BeforeEach(func() {
			crl = wrappa.ConcurrentRequestLimitFlag{}
		})

		It("parses the API action and limit", func() {
			crl.UnmarshalFlag("ListAllJobs=3")
			Expect(crl.Action).To(Equal("ListAllJobs"), "wrong action")
			Expect(crl.Limit).To(Equal(3), "wrong limit")
		})

		It("returns an error when the flag has no equals sign", func() {
			err := crl.UnmarshalFlag("banana")
			Expect(err).To(
				MatchError(
					"invalid concurrent request limit " +
						"'banana': value must be an assignment",
				),
			)
		})

		It("returns an error when the flag has multiple equals signs", func() {
			err := crl.UnmarshalFlag("foo=bar=baz")
			Expect(err).To(
				MatchError(
					"invalid concurrent request limit " +
						"'foo=bar=baz': value must be an assignment",
				),
			)
		})

		It("returns an error when the limit is not an integer", func() {
			err := crl.UnmarshalFlag("ListAllJobs=foo")
			Expect(err).To(
				MatchError(
					"invalid concurrent request limit " +
						"'ListAllJobs=foo': limit must be an integer",
				),
			)
		})
	})

	Describe("ConcurrentRequestPolicy#Validate", func() {
		It("raises an error when an invalid action is specified", func() {
			policy := wrappa.NewConcurrentRequestPolicy(
				[]wrappa.ConcurrentRequestLimitFlag{
					wrappa.ConcurrentRequestLimitFlag{
						Action: "InvalidAction",
						Limit:  0,
					},
				},
				[]string{atc.CreateJobBuild},
			)

			err := policy.Validate()

			Expect(err).To(MatchError(
				"invalid concurrent request limit " +
					"'InvalidAction=0': " +
					"'InvalidAction' is not a valid action",
			))
		})

		It("raises an error when the action is not supported", func() {
			policy := wrappa.NewConcurrentRequestPolicy(
				[]wrappa.ConcurrentRequestLimitFlag{
					wrappa.ConcurrentRequestLimitFlag{
						Action: atc.ListAllJobs,
						Limit:  0,
					},
				},
				[]string{atc.CreateJobBuild},
			)

			err := policy.Validate()

			Expect(err).To(MatchError(
				"invalid concurrent request limit " +
					"'ListAllJobs=0': " +
					"'ListAllJobs' is not supported",
			))
		})

		It("raises an error with multiple limits on the same action", func() {
			policy := wrappa.NewConcurrentRequestPolicy(
				[]wrappa.ConcurrentRequestLimitFlag{
					wrappa.ConcurrentRequestLimitFlag{
						Action: atc.CreateJobBuild,
						Limit:  0,
					},
					wrappa.ConcurrentRequestLimitFlag{
						Action: atc.CreateJobBuild,
						Limit:  0,
					},
				},
				[]string{atc.CreateJobBuild},
			)

			err := policy.Validate()

			Expect(err).To(MatchError(
				"invalid concurrent request limits: " +
					"multiple limits on 'CreateJobBuild'",
			))
		})
	})
})
