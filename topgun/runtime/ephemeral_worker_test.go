package topgun_test

import (
	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ephemeral Workers", func() {
	Context("with an ephemeral worker available", func() {
		BeforeEach(func() {
			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/worker-instances.yml",
				"-v", "worker_instances=2",
				"-o", "operations/ephemeral-worker.yml",
			)
		})

		Context("when the worker goes away", func() {
			BeforeEach(func() {
				Bosh("ssh", "worker/0", "-c", "sudo /var/vcap/bosh/bin/monit stop worker")
			})

			AfterEach(func() {
				Bosh("ssh", "worker/0", "-c", "sudo /var/vcap/bosh/bin/monit start worker")
			})

			It("disappears without stalling", func() {
				Eventually(func() int {
					workers := FlyTable("workers")
					return len(workers)
				}).Should(Equal(1))
			})
		})
	})
})
