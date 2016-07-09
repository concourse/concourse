package db_test

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"
	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
)

var _ = Describe("SQL DB", func() {
	var dbConn db.Conn
	var listener *pq.Listener

	var database db.DB

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		database = db.NewSQL(dbConn, bus)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("CreatePipe", func() {
		It("saves a pipe to the db", func() {
			myGuid, err := uuid.NewV4()
			Expect(err).NotTo(HaveOccurred())

			err = database.CreatePipe(myGuid.String(), "a-url")
			Expect(err).NotTo(HaveOccurred())

			pipe, err := database.GetPipe(myGuid.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(pipe.ID).To(Equal(myGuid.String()))
			Expect(pipe.URL).To(Equal("a-url"))
		})
	})

	It("saves and propagates events correctly", func() {
		build, err := database.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())
		Expect(build.Name).To(Equal("1"))

		By("allowing you to subscribe when no events have yet occurred")
		events, err := database.GetBuildEvents(build.ID, 0)
		Expect(err).NotTo(HaveOccurred())

		defer events.Close()

		By("saving them in order")
		err = database.SaveBuildEvent(build.ID, 0, event.Log{
			Payload: "some ",
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(events.Next()).To(Equal(envelope(event.Log{
			Payload: "some ",
		})))

		err = database.SaveBuildEvent(build.ID, 0, event.Log{
			Payload: "log",
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(events.Next()).To(Equal(envelope(event.Log{
			Payload: "log",
		})))

		By("allowing you to subscribe from an offset")
		eventsFrom1, err := database.GetBuildEvents(build.ID, 1)
		Expect(err).NotTo(HaveOccurred())

		defer eventsFrom1.Close()

		Expect(eventsFrom1.Next()).To(Equal(envelope(event.Log{
			Payload: "log",
		})))

		By("notifying those waiting on events as soon as they're saved")
		nextEvent := make(chan event.Envelope)
		nextErr := make(chan error)

		go func() {
			event, err := events.Next()
			if err != nil {
				nextErr <- err
			} else {
				nextEvent <- event
			}
		}()

		Consistently(nextEvent).ShouldNot(Receive())
		Consistently(nextErr).ShouldNot(Receive())

		err = database.SaveBuildEvent(build.ID, 0, event.Log{
			Payload: "log 2",
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(nextEvent).Should(Receive(Equal(envelope(event.Log{
			Payload: "log 2",
		}))))

		By("returning ErrBuildEventStreamClosed for Next calls after Close")
		events3, err := database.GetBuildEvents(build.ID, 0)
		Expect(err).NotTo(HaveOccurred())

		err = events3.Close()
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
			_, err := events3.Next()
			return err
		}).Should(Equal(db.ErrBuildEventStreamClosed))
	})

	It("saves and emits status events", func() {
		build, err := database.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())
		Expect(build.Name).To(Equal("1"))

		By("allowing you to subscribe when no events have yet occurred")
		events, err := database.GetBuildEvents(build.ID, 0)
		Expect(err).NotTo(HaveOccurred())

		defer events.Close()

		By("emitting a status event when started")
		started, err := database.StartBuild(build.ID, build.PipelineID, "engine", "metadata")
		Expect(err).NotTo(HaveOccurred())
		Expect(started).To(BeTrue())

		startedBuild, found, err := database.GetBuild(build.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(events.Next()).To(Equal(envelope(event.Status{
			Status: atc.StatusStarted,
			Time:   startedBuild.StartTime.Unix(),
		})))

		By("emitting a status event when finished")
		err = database.FinishBuild(build.ID, build.PipelineID, db.StatusSucceeded)
		Expect(err).NotTo(HaveOccurred())

		finishedBuild, found, err := database.GetBuild(build.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(events.Next()).To(Equal(envelope(event.Status{
			Status: atc.StatusSucceeded,
			Time:   finishedBuild.EndTime.Unix(),
		})))

		By("ending the stream when finished")
		_, err = events.Next()
		Expect(err).To(Equal(db.ErrEndOfBuildEventStream))
	})

	Describe("DeleteBuildEventsByBuildIDs", func() {
		It("deletes all build logs corresponding to the given build ids", func() {
			build1, err := database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = database.SaveBuildEvent(build1.ID, 0, event.Log{
				Payload: "log 1",
			})
			Expect(err).NotTo(HaveOccurred())

			build2, err := database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = database.SaveBuildEvent(build2.ID, 0, event.Log{
				Payload: "log 2",
			})
			Expect(err).NotTo(HaveOccurred())

			build3, err := database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = database.FinishBuild(build3.ID, 0, db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			err = database.FinishBuild(build1.ID, 0, db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			err = database.FinishBuild(build2.ID, 0, db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			build4, err := database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			By("doing nothing if the list is empty")
			err = database.DeleteBuildEventsByBuildIDs([]int{})
			Expect(err).NotTo(HaveOccurred())

			By("not returning an error")
			database.DeleteBuildEventsByBuildIDs([]int{build3.ID, build4.ID, build1.ID})
			Expect(err).NotTo(HaveOccurred())

			err = database.FinishBuild(build4.ID, 0, db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			By("deleting events for build 1")

			events1, err := database.GetBuildEvents(build1.ID, 0)
			Expect(err).NotTo(HaveOccurred())
			defer events1.Close()

			_, err = events1.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("preserving events for build 2")

			events2, err := database.GetBuildEvents(build2.ID, 0)
			Expect(err).NotTo(HaveOccurred())
			defer events2.Close()

			build2Event1, err := events2.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(build2Event1).To(Equal(envelope(event.Log{
				Payload: "log 2",
			})))

			_, err = events2.Next() // finish event
			Expect(err).NotTo(HaveOccurred())

			_, err = events2.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("deleting events for build 3")

			events3, err := database.GetBuildEvents(build3.ID, 0)
			Expect(err).NotTo(HaveOccurred())
			defer events3.Close()

			_, err = events3.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("being unflapped by build 4, which had no events at the time")

			events4, err := database.GetBuildEvents(build4.ID, 0)
			Expect(err).NotTo(HaveOccurred())
			defer events4.Close()

			_, err = events4.Next() // finish event
			Expect(err).NotTo(HaveOccurred())

			_, err = events4.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("updating ReapTime for the affected builds")

			reapedBuild1, found, err := database.GetBuild(build1.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(reapedBuild1.ReapTime).To(BeTemporally(">", reapedBuild1.EndTime))

			reapedBuild2, found, err := database.GetBuild(build2.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(reapedBuild2.ReapTime).To(BeZero())

			reapedBuild3, found, err := database.GetBuild(build3.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(reapedBuild3.ReapTime).To(Equal(reapedBuild1.ReapTime))

			reapedBuild4, found, err := database.GetBuild(build4.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			// Not required behavior, just a sanity check for what I think will happen
			Expect(reapedBuild4.ReapTime).To(Equal(reapedBuild1.ReapTime))
		})
	})
})

func envelope(ev atc.Event) event.Envelope {
	payload, err := json.Marshal(ev)
	Expect(err).ToNot(HaveOccurred())

	data := json.RawMessage(payload)

	return event.Envelope{
		Event:   ev.EventType(),
		Version: ev.Version(),
		Data:    &data,
	}
}
