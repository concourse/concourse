package topgun_test

import (
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
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
			var containerID int
			var contanerHandle string
			err := psql.Select("id, handle").
				From("containers").
				Where(sq.NotEq{"resource_config_id": nil}).
				RunWith(dbConn).
				QueryRow().
				Scan(&containerID, &contanerHandle)
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("eventually expiring the resource config container: %s", contanerHandle))
			Eventually(func() int {
				var containerNum int
				err := psql.Select("COUNT(id)").From("containers").Where(sq.Eq{"id": containerID}).RunWith(dbConn).QueryRow().Scan(&containerNum)
				Expect(err).ToNot(HaveOccurred())
				return containerNum
			}, 10*time.Minute, time.Second).Should(BeZero())

			By("checking resource again")
			fly("check-resource", "-r", "volume-gc-test/tick-tock")

			By("getting the resource config container")
			var newContainerID int
			err = psql.Select("id").
				From("containers").
				Where(sq.NotEq{"resource_config_id": nil}).
				RunWith(dbConn).
				QueryRow().
				Scan(&newContainerID)
			Expect(err).ToNot(HaveOccurred())
			Expect(newContainerID).NotTo(Equal(containerID))
		})
	})
})
