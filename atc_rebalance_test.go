package topgun_test

import (
	"strings"
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ATC Rebalance", func() {
	Context("with two atcs available", func() {
		var atcs []boshInstance
		var atc0IP string
		var atc1IP string

		BeforeEach(func() {
			By("Configuring two ATCs")
			Deploy("deployments/concourse-two-atcs-slow-tracking.yml")
			waitForRunningWorker()

			atcs = JobInstances("atc")
			atc0IP = atcs[0].IP
			atc1IP = atcs[1].IP

			atc0URL := "http://" + atcs[0].IP + ":8080"
			fly("login", "-c", atc0URL, "-u", atcUsername, "-p", atcPassword)
		})

		Describe("when a rebalance time is configured", func() {
			It("the worker eventually connects to both web nodes over a period of time", func() {
				Eventually(func() string {
					workers := flyTable("workers", "-d")
					return strings.Split(workers[0]["garden address"], ":")[0]
				}, time.Second*60, time.Second*5).Should(Equal(atc0IP))
				Eventually(func() string {
					workers := flyTable("workers", "-d")
					return strings.Split(workers[0]["garden address"], ":")[0]
				}, time.Minute*5, time.Second*5).Should(Equal(atc1IP))
			})
		})
	})
})
