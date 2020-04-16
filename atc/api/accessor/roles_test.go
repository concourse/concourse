package accessor_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
)

var _ = Describe("Roles", func() {

	It("has a role for every route", func() {
		for _, route := range atc.Routes {
			role, found := accessor.DefaultRoles[route.Name]

			if !found {
				role, found = accessor.AdminRoles[route.Name]
			}

			Expect(found).To(BeTrue())
			Expect(role).NotTo(BeEmpty())
		}
	})
})
