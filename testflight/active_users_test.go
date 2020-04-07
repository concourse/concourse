package testflight_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("tracking active users", func() {
	It("shows that you have done some stuff", func() {
		activeUsers := fly("active-users", "--print-table-headers")
		Expect(activeUsers.Out).To(gbytes.Say(`username\s+connector\s+last login`))
		Expect(activeUsers.Out).To(gbytes.Say(fmt.Sprintf(`%s\s+%s\s+%s`,
			config.ATCUsername,
			"local",
			time.Now().Format("2006-01-02"),
		)))
	})
})
