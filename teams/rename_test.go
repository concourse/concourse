package teams_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Renaming a team", func() {
	var (
		newTeamName string
		oldTeamName string
	)
	BeforeEach(func() {
		newTeamName = fmt.Sprintf("renamed-team-%d", GinkgoParallelNode())
	})

	Context("with a team that exists", func() {
		BeforeEach(func() {
			oldTeamName = fmt.Sprintf("old-team-%d", GinkgoParallelNode())
			setTeam(oldTeamName)
		})

		AfterEach(func() {
			destroyTeam(newTeamName)
			destroyTeam(oldTeamName)
		})

		It("renames the team", func() {
			renameTeam(oldTeamName, newTeamName)
			teams := listTeams()

			Expect(string(teams)).Should(ContainSubstring(newTeamName))
		})
	})

})
