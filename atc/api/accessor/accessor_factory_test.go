package accessor_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/db"
)

var _ = Describe("AccessorFactory", func() {
	var (
		systemClaimKey    string
		systemClaimValues []string

		role         string
		verification accessor.Verification
		teams        []db.Team
	)

	BeforeEach(func() {
		systemClaimKey = "sub"
		systemClaimValues = []string{"some-sub"}

		role = "some-role"
		verification = accessor.Verification{}
		teams = []db.Team{}
	})

	Describe("Create", func() {

		var access accessor.Access

		JustBeforeEach(func() {
			factory := accessor.NewAccessFactory(systemClaimKey, systemClaimValues)
			access = factory.Create(role, verification, teams)
		})

		It("creates an accessor", func() {
			Expect(access).NotTo(BeNil())
		})
	})
})
