package topgun_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	sq "github.com/Masterminds/squirrel"
	bgclient "github.com/concourse/baggageclaim/client"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe(":life volume gc", func() {
	var dbConn *sql.DB

	BeforeEach(func() {
		var err error
		dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", atcIP))
		Expect(err).ToNot(HaveOccurred())

		Deploy("deployments/single-vm.yml")
	})

	Describe("A volume that belonged to a container that is now gone", func() {
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
				err = psql.Select("COUNT(id)").From("containers").Where(sq.Eq{"id": containerIDs}).RunWith(dbConn).QueryRow().Scan(&containerNum)
				Expect(err).ToNot(HaveOccurred())

				if containerNum == len(containerIDs) {
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
			bcClient := bgclient.New(fmt.Sprintf("http://%s:7788", atcIP), http.DefaultTransport)

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

	Describe("A volume that belonged to a resource cache that is no longer in use", func() {
		It("is removed from the database and worker [#129726933]", func() {
			By("setting pipeline that creates resource cache")
			fly("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "volume-gc-test")

			By("triggering job")
			fly("trigger-job", "-w", "-j", "volume-gc-test/simple-job")

			By("getting resource cache")
			var resourceCacheID int
			err := psql.Select("rch.id").
				From("resource_caches rch").
				LeftJoin("resource_configs rcf ON rch.resource_config_id = rcf.id").
				LeftJoin("base_resource_types brt ON rcf.base_resource_type_id = brt.id").
				Where("brt.name = 'time'").
				RunWith(dbConn).
				QueryRow().Scan(&resourceCacheID)
			Expect(err).ToNot(HaveOccurred())

			By("getting volume for resource cache")
			var volumeID int
			var volumeHandle string
			err = psql.Select("id, handle").
				From("volumes").
				Where(sq.Eq{"resource_cache_id": resourceCacheID}).
				RunWith(dbConn).
				QueryRow().
				Scan(&volumeID, &volumeHandle)
			Expect(err).ToNot(HaveOccurred())

			By("creating a new version of resource")
			fly("check-resource", "-r", "volume-gc-test/tick-tock")

			By(fmt.Sprintf("eventually expiring the resource cache: %d", resourceCacheID))
			Eventually(func() int {
				var resourceCacheNum int
				err := psql.Select("COUNT(id)").From("resource_caches").Where(sq.Eq{"id": resourceCacheID}).RunWith(dbConn).QueryRow().Scan(&resourceCacheNum)
				Expect(err).ToNot(HaveOccurred())

				var volumeNum int
				err = psql.Select("COUNT(id)").From("volumes").Where(sq.Eq{"id": volumeID}).RunWith(dbConn).QueryRow().Scan(&volumeNum)
				Expect(err).ToNot(HaveOccurred())

				if resourceCacheNum == 1 {
					By(fmt.Sprintf("not expiring volume so long as its resource cache is there"))
					Expect(volumeNum).To(Equal(1))
				}

				return resourceCacheNum
			}, 10*time.Minute, time.Second).Should(BeZero())

			By(fmt.Sprintf("eventually expiring the resource cache volumes: %d", resourceCacheID))
			Eventually(func() int {
				var volumeNum int
				err := psql.Select("COUNT(id)").From("volumes").Where(sq.Eq{"id": volumeID}).RunWith(dbConn).QueryRow().Scan(&volumeNum)
				Expect(err).ToNot(HaveOccurred())
				return volumeNum
			}, 10*time.Minute, time.Second).Should(BeZero())

			By("having removed the volumes from the worker")
			bcClient := bgclient.New(fmt.Sprintf("http://%s:7788", atcIP), http.DefaultTransport)

			volumes, err := bcClient.ListVolumes(logger, nil)
			Expect(err).ToNot(HaveOccurred())

			existingHandles := []string{}
			for _, v := range volumes {
				existingHandles = append(existingHandles, v.Handle())
			}

			Expect(existingHandles).ToNot(ContainElement(volumeHandle))
		})
	})
})
