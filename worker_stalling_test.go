package topgun_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("[#129726011] Worker stalling", func() {
	var dbConn *sql.DB

	BeforeEach(func() {
		var err error
		dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", atcIP))
		Expect(err).ToNot(HaveOccurred())
	})

	waitForStalledWorker := func() string {
		var stalledWorkerName string
		Eventually(func() string {
			rows, err := psql.Select("name, state").From("workers").RunWith(dbConn).Query()
			Expect(err).ToNot(HaveOccurred())

			for rows.Next() {
				var name string
				var state string

				err := rows.Scan(&name, &state)
				Expect(err).ToNot(HaveOccurred())

				if state != "stalled" {
					continue
				}

				if stalledWorkerName != "" {
					Fail("multiple workers stalled")
				}

				stalledWorkerName = name
			}

			return stalledWorkerName
		}).ShouldNot(BeEmpty())

		return stalledWorkerName
	}

	waitForWorkersToBeRunning := func() {
		Eventually(func() bool {
			rows, err := psql.Select("name, state").From("workers").RunWith(dbConn).Query()
			Expect(err).ToNot(HaveOccurred())

			anyStalled := false

			for rows.Next() {
				var name string
				var state string

				err := rows.Scan(&name, &state)
				Expect(err).ToNot(HaveOccurred())

				if state == "stalled" {
					anyStalled = true
				}
			}

			return anyStalled
		}).Should(BeFalse())
	}

	Context("with two workers available", func() {
		BeforeEach(func() {
			Deploy("deployments/two-forwarded-workers.yml")
		})

		It("initially runs tasks across all workers", func() {
			Eventually(func() map[string]struct{} {
				fly("execute", "-c", "tasks/tiny.yml")
				rows, err := psql.Select("id, worker_name").From("containers").RunWith(dbConn).Query()
				Expect(err).ToNot(HaveOccurred())

				usedWorkers := map[string]struct{}{}
				for rows.Next() {
					var id int
					var workerName string
					err := rows.Scan(&id, &workerName)
					Expect(err).ToNot(HaveOccurred())
					usedWorkers[workerName] = struct{}{}
				}

				return usedWorkers
			}, 10*time.Minute).Should(HaveLen(2))
		})

		Context("when one worker goes away", func() {
			var stalledWorkerName string

			BeforeEach(func() {
				bosh("ssh", "worker/0", "-c", "sudo /var/vcap/bosh/bin/monit stop beacon")
				bosh("ssh", "worker/0", "-c", "sudo /var/vcap/bosh/bin/monit stop garden")
				stalledWorkerName = waitForStalledWorker()
			})

			AfterEach(func() {
				bosh("ssh", "worker/0", "-c", "sudo /var/vcap/bosh/bin/monit start beacon")
				bosh("ssh", "worker/0", "-c", "sudo /var/vcap/bosh/bin/monit start garden")
				waitForWorkersToBeRunning()
			})

			It("enters 'stalled' state and is no longer used for new containers", func() {
				for i := 0; i < 10; i++ {
					fly("execute", "-c", "tasks/tiny.yml")
					rows, err := psql.Select("id, worker_name").From("containers").RunWith(dbConn).Query()
					Expect(err).ToNot(HaveOccurred())

					usedWorkers := map[string]struct{}{}
					for rows.Next() {
						var id int
						var workerName string
						err := rows.Scan(&id, &workerName)
						Expect(err).ToNot(HaveOccurred())
						usedWorkers[workerName] = struct{}{}
					}

					Expect(usedWorkers).To(HaveLen(1))
					Expect(usedWorkers).ToNot(ContainElement(stalledWorkerName))
				}
			})
		})
	})

	Context("with no other worker available", func() {
		BeforeEach(func() {
			Deploy("deployments/single-vm-forwarded-worker.yml")
		})

		Context("when the worker stalls while a build is running", func() {
			var stalledWorkerName string

			var buildSession *gexec.Session
			var buildID string

			BeforeEach(func() {
				buildSession = spawnFly("execute", "-c", "tasks/wait.yml")
				Eventually(buildSession).Should(gbytes.Say("executing build"))

				buildRegex := regexp.MustCompile(`executing build (\d+)`)
				matches := buildRegex.FindSubmatch(buildSession.Out.Contents())
				buildID = string(matches[1])

				Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

				By("stopping the beacon without draining")
				bosh("ssh", "concourse/0", "-c", "sudo /var/vcap/bosh/bin/monit stop beacon")
				bosh("ssh", "concourse/0", "-c", "sudo /var/vcap/bosh/bin/monit stop garden")

				By("waiting for it to stall")
				stalledWorkerName = waitForStalledWorker()
			})

			AfterEach(func() {
				buildSession.Signal(os.Interrupt)
				<-buildSession.Exited
			})

			Context("when the worker does not come back", func() {
				AfterEach(func() {
					bosh("ssh", "concourse/0", "-c", "sudo /var/vcap/bosh/bin/monit start beacon")
					bosh("ssh", "concourse/0", "-c", "sudo /var/vcap/bosh/bin/monit start garden")
					waitForWorkersToBeRunning()
				})

				It("does not fail the build", func() {
					Consistently(buildSession).ShouldNot(gexec.Exit())
				})
			})

			Context("when the worker comes back", func() {
				It("resumes the build", func() {
					bosh("ssh", "concourse/0", "-c", "sudo /var/vcap/bosh/bin/monit start beacon")
					bosh("ssh", "concourse/0", "-c", "sudo /var/vcap/bosh/bin/monit start garden")
					waitForWorkersToBeRunning()

					// Garden doesn't seem to stream output after restarting it. Guardian bug?
					// By("reattaching to the build")
					// _, err := ioutil.ReadAll(buildSession.Out)
					// Expect(err).ToNot(HaveOccurred())
					// Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

					By("hijacking the build to tell it to finish")
					Eventually(func() int {
						session := spawnFlyInteractive(
							bytes.NewBufferString("3\n"),
							"hijack",
							"-b", buildID,
							"-s", "one-off",
							"touch", "/tmp/stop-waiting",
						)

						<-session.Exited
						return session.ExitCode()
					}).Should(Equal(0))

					By("waiting for the build to exit")
					// Garden doesn't seem to stream output after restarting it. Guardian bug?
					// Eventually(buildSession).Should(gbytes.Say("done"))
					<-buildSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))
				})
			})
		})
	})
})
