package topgun_test

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe(":life Garbage collecting resource containers", func() {
	var dbConn *sql.DB

	BeforeEach(func() {
		var err error
		dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", atcIP))
		Expect(err).ToNot(HaveOccurred())

		Deploy("deployments/two-forwarded-workers.yml")
	})

	Describe("A container that is used by resource checking on freshly deployed worker", func() {
		It("is recreated in database and worker [#129726933]", func() {
			By("setting pipeline that creates resource cache")
			fly("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "volume-gc-test")

			By("checking resource")
			fly("check-resource", "-r", "volume-gc-test/tick-tock")

			By("getting the resource config container")
			containers := flyTable("containers")
			var checkContainerHandle string
			for _, container := range containers {
				if container["type"] == "check" {
					checkContainerHandle = container["handle"]
					break
				}
			}
			Expect(checkContainerHandle).NotTo(BeEmpty())

			By(fmt.Sprintf("eventually expiring the resource config container: %s", checkContainerHandle))
			Eventually(func() bool {
				containers := flyTable("containers")
				for _, container := range containers {
					if container["type"] == "check" {
						return true
					}
				}
				return false
			}, 10*time.Minute, time.Second).Should(BeFalse())

			By("checking resource again")
			fly("check-resource", "-r", "volume-gc-test/tick-tock")

			By("getting the resource config container")
			containers = flyTable("containers")
			var newCheckContainerHandle string
			for _, container := range containers {
				if container["type"] == "check" {
					newCheckContainerHandle = container["handle"]
					break
				}
			}
			Expect(newCheckContainerHandle).NotTo(Equal(checkContainerHandle))
		})
	})
})
