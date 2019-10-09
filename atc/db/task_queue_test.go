package db_test

import (
	"database/sql"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/db"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TaskQueue", func() {
	var (
		listener *pq.Listener
		dbConn   db.Conn

		logger *lagertest.TestLogger

		taskQueue db.TaskQueue
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())

		logger = lagertest.NewTestLogger("test")
		dbConn = postgresRunner.OpenConn()

		taskQueue = db.NewTaskQueue(dbConn)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("queue length", func() {
		Context("when the id is not on any queue", func() {
			It("returns that the length of the queue is 0", func() {
				length, err := taskQueue.Length("foo")
				Expect(length).To(BeZero())
				Expect(err).NotTo(HaveOccurred())
			})
		})
		Context("when the id is the only element in the queue", func() {
			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO tasks_queue(id, platform, team_id, worker_tag, insert_time) VALUES ('foo_id', 'foo_platform', 42, 'foo_tag', NOW())")
				Expect(err).ToNot(HaveOccurred())
			})
			It("returns that the length of the queue is 1", func() {
				length, err := taskQueue.Length("foo_id")
				Expect(length).To(Equal(1))
				Expect(err).ToNot(HaveOccurred())
			})
		})
		Context("when the id is the only element in the queue", func() {
			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO tasks_queue(id, platform, team_id, worker_tag, insert_time) VALUES ('foo_id', 'foo_platform', 42, 'foo_tag', NOW())")
				Expect(err).ToNot(HaveOccurred())
			})
			It("returns that the length of the queue where the element is in is 1", func() {
				length, err := taskQueue.Length("foo_id")
				Expect(length).To(Equal(1))
				Expect(err).ToNot(HaveOccurred())
			})
		})
		Context("when 3 elements are in the same queue", func() {
			BeforeEach(func() {
				for _, id := range []string{"foo", "bar", "baz"} {
					_, err := dbConn.Exec("INSERT INTO tasks_queue(id, platform, team_id, worker_tag, insert_time) VALUES ($1, 'foo_platform', 42, 'foo_tag', NOW())", id)
					Expect(err).ToNot(HaveOccurred())
				}
			})
			It("returns that the length of the queue where one of the element is in is 3", func() {
				length, err := taskQueue.Length("bar")
				Expect(length).To(Equal(3))
				Expect(err).ToNot(HaveOccurred())
			})
			It("returns that the length of the queue for a different element is 0", func() {
				length, err := taskQueue.Length("blah")
				Expect(length).To(Equal(0))
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("queue position", func() {
		Context("when an element is not yet in the queue", func() {
			It("returns that its position is 0", func() {
				pos, err := taskQueue.Position("foo")
				Expect(pos).To(Equal(0))
				Expect(err).ToNot(HaveOccurred())
			})
		})
		Context("when the id is the only element in the queue", func() {
			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO tasks_queue(id, platform, team_id, worker_tag, insert_time) VALUES ('foo_id', 'foo_platform', 42, 'foo_tag', NOW())")
				Expect(err).ToNot(HaveOccurred())
			})
			It("returns that its position is 1", func() {
				pos, err := taskQueue.Position("foo_id")
				Expect(pos).To(Equal(1))
				Expect(err).ToNot(HaveOccurred())
			})
		})
		Context("when 3 elements are in the same queue", func() {
			BeforeEach(func() {
				for _, id := range []string{"foo", "bar", "baz"} {
					_, err := dbConn.Exec("INSERT INTO tasks_queue(id, platform, team_id, worker_tag, insert_time) VALUES ($1, 'foo_platform', 42, 'foo_tag', NOW())", id)
					Expect(err).ToNot(HaveOccurred())
				}
				_, err := dbConn.Exec("INSERT INTO tasks_queue(id, platform, team_id, worker_tag, insert_time) VALUES ('extraneous', 'ext_platform', 21, 'foo_tag', NOW())")
				Expect(err).ToNot(HaveOccurred())
			})
			It("returns that the position of the second element is 2", func() {
				pos, err := taskQueue.Position("bar")
				Expect(pos).To(Equal(2))
				Expect(err).ToNot(HaveOccurred())
			})
			It("returns that the position of an element in a different queue is 1", func() {
				length, err := taskQueue.Length("extraneous")
				Expect(length).To(Equal(1))
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Find queue", func() {
		Context("when an element is already present in a queue", func() {
			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO tasks_queue(id, platform, team_id, worker_tag, insert_time) VALUES ('foo_id', 'foo_platform', 42, 'foo_tag', NOW())")
				Expect(err).ToNot(HaveOccurred())
			})
			It("is found in the correct queue", func() {
				platform, team_id, worker_tag, err := taskQueue.FindQueue("foo_id")
				Expect(err).ToNot(HaveOccurred())
				Expect(platform).To(Equal("foo_platform"))
				Expect(team_id).To(Equal(42))
				Expect(worker_tag).To(Equal("foo_tag"))
			})
		})
		Context("when an element is in no queue", func() {
			It("returns an error", func() {
				_, _, _, err := taskQueue.FindQueue("foo_id")
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(sql.ErrNoRows))
			})
		})
	})

	Describe("Find or append to queue", func() {
		Context("when an element is added to an empty queue for the first time", func() {
			BeforeEach(func() {
				pos, length, err := taskQueue.FindOrAppend("foo", "foo_platform", 42, "foo_tag", logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(pos).To(Equal(1))
				Expect(length).To(Equal(1))
			})
			It("appears in the database", func() {
				rows, err := dbConn.Query(`SELECT id FROM tasks_queue`)
				Expect(err).NotTo(HaveOccurred())
				for rows.Next() {
					var id string
					err := rows.Scan(&id)
					Expect(err).NotTo(HaveOccurred())
					Expect(id).To(Equal("foo"))
				}
			})
		})
		Context("when an element is appended to a queue with three elements", func() {
			BeforeEach(func() {
				for _, id := range []string{"foo", "bar", "baz"} {
					_, err := dbConn.Exec("INSERT INTO tasks_queue(id, platform, team_id, worker_tag, insert_time) VALUES ($1, 'foo_platform', 42, 'foo_tag', NOW())", id)
					Expect(err).ToNot(HaveOccurred())
				}
				pos, length, err := taskQueue.FindOrAppend("blah", "foo_platform", 42, "foo_tag", logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(pos).To(Equal(4))
				Expect(length).To(Equal(4))
			})
			It("appears in the database", func() {
				rows, err := dbConn.Query(`SELECT id FROM tasks_queue where id = 'blah'`)
				Expect(err).NotTo(HaveOccurred())
				for rows.Next() {
					var id string
					err := rows.Scan(&id)
					Expect(err).NotTo(HaveOccurred())
					Expect(id).ToNot(BeNil())
				}
			})
			Context("when a new element is added to a different queue", func() {
				It("does not interfere with the existing queue", func() {
					pos, length, err := taskQueue.FindOrAppend("new_foo", "blah_platform", 42, "foo_tag", logger)
					Expect(err).ToNot(HaveOccurred())
					Expect(pos).To(Equal(1))
					Expect(length).To(Equal(1))

					var old_queue_len int
					err = dbConn.QueryRow(`SELECT COUNT(*) FROM tasks_queue where platform = 'foo_platform'`).Scan(&old_queue_len)
					Expect(err).ToNot(HaveOccurred())
					Expect(old_queue_len).To(Equal(4))
				})
			})
		})
		Context("when an element already exist in a queue", func() {
			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO tasks_queue(id, platform, team_id, worker_tag, insert_time) VALUES ('foo_id', 'foo_platform', 42, 'foo_tag', NOW())")
				Expect(err).ToNot(HaveOccurred())
			})
			Context("when the same element is added to a different queue", func() {
				It("is removed from the existing queue, logging a warning", func() {
					pos, length, err := taskQueue.FindOrAppend("foo_id", "diff_plat", 42, "foo_tag", logger)
					Expect(err).ToNot(HaveOccurred())
					Expect(pos).To(Equal(1))
					Expect(length).To(Equal(1))
					Expect(logger.LogMessages()).To(ContainElement("test.foo_id.already-present-in-different-queue"))
				})
			})
		})
	})

	Describe("Dequeue", func() {
		Context("when an element is removed from the queue", func() {
			Context("if the element is present", func() {
				BeforeEach(func() {
					_, err := dbConn.Exec("INSERT INTO tasks_queue(id, platform, team_id, worker_tag, insert_time) VALUES ('foo_id', 'foo_platform', 42, 'foo_tag', NOW())")
					Expect(err).ToNot(HaveOccurred())
				})
				It("is removed from the queue", func() {
					taskQueue.Dequeue("foo_id", logger)
					Expect(logger.LogMessages()).To(Equal([]string{}))
				})
			})
		})
	})
})
