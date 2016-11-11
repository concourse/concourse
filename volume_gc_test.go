package topgun_test

import (
	"database/sql"
	"fmt"
	"time"

	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	sq "github.com/Masterminds/squirrel"
	bgclient "github.com/concourse/baggageclaim/client"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A volume that belonged to a container that is now gone", func() {
	var dbConn *sql.DB

	BeforeEach(func() {
		var err error
		dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", atcIP))
		Expect(err).ToNot(HaveOccurred())

		Deploy("deployments/single-vm.yml")
	})

	It("is removed from the database and worker [#129726011]", func() {
		By("running a task with container having a rootfs, input, and output volume")
		fly("execute", "-c", "tasks/input-output.yml", "-i", "some-input=./tasks")

		By("collecting the build containers")
		rows, err := psql.Select("id, handle").From("containers").Where(sq.Eq{"build_id": 1}).RunWith(dbConn).Query()
		Expect(err).ToNot(HaveOccurred())

		buildContainers := map[int]string{}
		containerIDs := []int{}
		for rows.Next() {
			var id int
			var handle string
			err := rows.Scan(&id, &handle)
			Expect(err).ToNot(HaveOccurred())

			buildContainers[id] = handle
			containerIDs = append(containerIDs, id)
		}

		By("collecting the container volumes")
		rows, err = psql.Select("id, handle").From("volumes").Where(sq.Eq{"container_id": containerIDs}).RunWith(dbConn).Query()
		Expect(err).ToNot(HaveOccurred())

		containerVolumes := map[int]string{}
		volumeIDs := []int{}
		for rows.Next() {
			var id int
			var handle string
			err := rows.Scan(&id, &handle)
			Expect(err).ToNot(HaveOccurred())

			containerVolumes[id] = handle
			volumeIDs = append(volumeIDs, id)
		}

		By(fmt.Sprintf("eventually expiring the build containers: %v", containerIDs))
		Eventually(func() int {
			var volNum int
			err := psql.Select("COUNT(id)").From("volumes").Where(sq.Eq{"id": volumeIDs}).RunWith(dbConn).QueryRow().Scan(&volNum)
			Expect(err).ToNot(HaveOccurred())

			var containerNum int
			err = psql.Select("COUNT(id)").From("containers").Where(sq.Eq{"build_id": 1}).RunWith(dbConn).QueryRow().Scan(&containerNum)
			Expect(err).ToNot(HaveOccurred())

			if containerNum > 0 {
				By(fmt.Sprintf("not expiring volumes so long as their containers are there (%d remaining)", containerNum))
				Expect(volNum).To(Equal(len(volumeIDs)))
			}

			return containerNum
		}, 10*time.Minute, time.Second).Should(BeZero())

		By("having removed the containers from the worker")
		gClient := gclient.New(gconn.New("tcp", fmt.Sprintf("%s:7777", atcIP)))

		containers, err := gClient.Containers(nil)
		Expect(err).ToNot(HaveOccurred())

		existingHandles := []string{}
		for _, c := range containers {
			existingHandles = append(existingHandles, c.Handle())
		}

		for _, handle := range buildContainers {
			Expect(existingHandles).ToNot(ContainElement(handle))
		}

		By(fmt.Sprintf("eventually expiring the container volumes: %v", volumeIDs))
		Eventually(func() int {
			var num int
			err := psql.Select("COUNT(id)").From("volumes").Where(sq.Eq{"id": volumeIDs}).RunWith(dbConn).QueryRow().Scan(&num)
			Expect(err).ToNot(HaveOccurred())
			return num
		}, 10*time.Minute, time.Second).Should(BeZero())

		By("having removed the volumes from the worker")
		bcClient := bgclient.New(fmt.Sprintf("http://%s:7788", atcIP))

		volumes, err := bcClient.ListVolumes(logger, nil)
		Expect(err).ToNot(HaveOccurred())

		existingHandles = []string{}
		for _, v := range volumes {
			existingHandles = append(existingHandles, v.Handle())
		}

		for _, handle := range containerVolumes {
			Expect(existingHandles).ToNot(ContainElement(handle))
		}
	})
})
