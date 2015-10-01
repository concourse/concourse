package group_test

import (
	"github.com/concourse/atc"
	. "github.com/concourse/atc/web/group"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("States", func() {
	Describe("UnhighlightedStates", func() {
		It("returns a list of states where none are enabled", func() {
			config := atc.GroupConfigs{
				{Name: "first-group"},
				{Name: "second-group"},
			}
			states := UnhighlightedStates(config)

			Expect(states).To(ConsistOf(
				State{"first-group", false},
				State{"second-group", false},
			))

		})
	})
})
