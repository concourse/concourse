package migration_test

import (
	"database/sql"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Separate worker_task_caches table", func() {
	const preMigrationVersion = 1556724983
	const postMigrationVersion = 1557152441

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("separate worker_task_caches table", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			setup_for_up_test(db)

			_ = db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			rows, err := db.Query(`SELECT id, job_id, step_name, path FROM task_caches`)
			Expect(err).NotTo(HaveOccurred())

			taskCaches := make(map[int]string)

			for rows.Next() {
				var id int
				var job_id int
				var step_name string
				var path string

				err := rows.Scan(&id, &job_id, &step_name, &path)
				Expect(err).NotTo(HaveOccurred())

				taskCaches[id] = strings.Join([]string{strconv.Itoa(job_id), step_name, path}, ",")
			}

			Expect(taskCaches[1]).To(Equal("1,some-step,some-path"))
			Expect(taskCaches[3]).To(Equal("2,some-step,some-path"))

			rows, err = db.Query(`SELECT id, worker_name, task_cache_id FROM worker_task_caches`)
			Expect(err).NotTo(HaveOccurred())

			workerTaskCaches := make(map[int]string)

			for rows.Next() {
				var id int
				var worker_name string
				var task_cache_id int

				err := rows.Scan(&id, &worker_name, &task_cache_id)
				Expect(err).NotTo(HaveOccurred())

				workerTaskCaches[id] = strings.Join([]string{worker_name, strconv.Itoa(task_cache_id)}, ",")
			}

			Expect(workerTaskCaches[1]).To(Equal("some-worker,1"))
			Expect(workerTaskCaches[2]).To(Equal("some-worker,3"))
			Expect(workerTaskCaches[3]).To(Equal("some-other-worker,1"))

			_ = db.Close()

		})
	})

	Context("down", func() {
		It("merge worker_task_caches and task_caches table", func() {
			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			setup_for_down_test(db)

			_ = db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			rows, err := db.Query(`SELECT id, worker_name, job_id, step_name, path FROM worker_task_caches`)
			Expect(err).NotTo(HaveOccurred())

			workerTaskCaches := make(map[int]string)

			for rows.Next() {
				var id int
				var worker_name string
				var job_id int
				var step_name string
				var path string

				err := rows.Scan(&id, &worker_name, &job_id, &step_name, &path)
				Expect(err).NotTo(HaveOccurred())

				workerTaskCaches[id] = strings.Join([]string{worker_name, strconv.Itoa(job_id), step_name, path}, ",")
			}

			Expect(workerTaskCaches[1]).To(Equal("some-worker,1,some-step,some-path"))
			Expect(workerTaskCaches[2]).To(Equal("some-worker,2,some-step,some-path"))
			Expect(workerTaskCaches[3]).To(Equal("some-other-worker,1,some-step,some-path"))

			_ = db.Close()
		})
	})
})

func setup_for_down_test(db *sql.DB) {
	_, err := db.Exec(`
			INSERT INTO teams(id, name) VALUES
			(1, 'some-team')
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
			INSERT INTO workers(name) VALUES
			('some-worker'),
			('some-other-worker')
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
			INSERT INTO pipelines(id, team_id, name) VALUES
			(1, 1, 'pipeline1')
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
				INSERT INTO jobs(id, pipeline_id, name, config) VALUES
					(1, 1, 'job1', '{"name":"job1"}'),
					(2, 1, 'job2', '{"name":"job2"}')
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
			INSERT INTO task_caches(id, job_id, step_name, path) VALUES
			(1, 1, 'some-step', 'some-path'),
			(2, 2, 'some-step', 'some-path')
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
			INSERT INTO worker_task_caches(id, worker_name, task_cache_id) VALUES
			(1, 'some-worker', 1),
			(2, 'some-worker', 2),
			(3, 'some-other-worker', 1)
			`)
	Expect(err).NotTo(HaveOccurred())
}

func setup_for_up_test(db *sql.DB) {
	_, err := db.Exec(`
			INSERT INTO teams(id, name) VALUES
			(1, 'some-team')
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
			INSERT INTO workers(name) VALUES
			('some-worker'),
			('some-other-worker')
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
			INSERT INTO pipelines(id, team_id, name) VALUES
			(1, 1, 'pipeline1')
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
				INSERT INTO jobs(id, pipeline_id, name, config) VALUES
					(1, 1, 'job1', '{"name":"job1"}'),
					(2, 1, 'job2', '{"name":"job2"}')
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
			INSERT INTO worker_task_caches(id, worker_name, job_id, step_name, path) VALUES
			(1, 'some-worker', 1, 'some-step', 'some-path'),
			(2, 'some-worker', 2, 'some-step', 'some-path'),
			(3, 'some-other-worker', 1, 'some-step', 'some-path')
			`)
	Expect(err).NotTo(HaveOccurred())
}
