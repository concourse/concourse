package web_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"
)

var _ = Describe("NoPipelines", func() {
	It("can view the all builds page with no pipelines configured", func() {
		Expect(page.Navigate(atcURL)).To(Succeed())
		Eventually(page.Find(".nav-right .nav-item a")).Should(BeFound())
		Expect(page.Find(".nav-right .nav-item a").Click()).To(Succeed())
		Eventually(page).Should(HaveURL(atcRoute("/builds")))
	})
})
