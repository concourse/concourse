package db_test

import (
	"context"
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildEventStore", func() {
	var eventStore db.EventStore

	BeforeEach(func() {
		eventStore = db.NewBuildEventStore(dbConn)
	})

	Describe("Setup", func() {
		BeforeEach(func() {
			_, err := dbConn.Exec(`
				DROP SCHEMA public CASCADE;
				CREATE SCHEMA public;
			`)
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Setup(context.TODO())
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates the build_events table", func() {
			var exists bool
			err := dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'build_events')").Scan(&exists)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue(), "didn't create 'build_events' table")
		})

		It("creates the correct columns", func() {
			type schemaColumn struct {
				name     string
				dataType string
			}

			rows, err := dbConn.Query("SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'build_events'")
			Expect(err).ToNot(HaveOccurred())
			var columns []schemaColumn
			for rows.Next() {
				var column schemaColumn
				err = rows.Scan(&column.name, &column.dataType)
				Expect(err).ToNot(HaveOccurred())
				columns = append(columns, column)
			}
			Expect(columns).To(ConsistOf(
				schemaColumn{name: "build_id", dataType: "integer"},
				schemaColumn{name: "type", dataType: "character varying"},
				schemaColumn{name: "payload", dataType: "text"},
				schemaColumn{name: "event_id", dataType: "integer"},
				schemaColumn{name: "version", dataType: "text"},
			))
		})

		It("creates the required indexes", func() {
			var exists bool
			err := dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'build_events_build_id_idx')").Scan(&exists)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue(), "didn't create 'build_events_build_id_idx' index")

			err = dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'build_events_build_id_event_id')").Scan(&exists)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue(), "didn't create 'build_events_build_id_event_id' index")
		})

		It("doesn't error when run multiple times", func() {
			err := eventStore.Setup(context.TODO())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Initialize", func() {
		var build db.Build

		BeforeEach(func() {
			err := eventStore.Setup(context.TODO())
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			err := eventStore.Initialize(context.TODO(), build)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the build is a pipeline build", func() {
			BeforeEach(func() {
				var err error
				build, err = defaultPipeline.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())
			})

			It("creates a pipeline build events table", func() {
				tableName := fmt.Sprintf("pipeline_build_events_%d", defaultPipeline.ID())

				var exists bool
				err := dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = '" + tableName + "')").Scan(&exists)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue(), "didn't create '" + tableName + "' table")
			})

			It("creates the required indexes", func() {
				tableName := fmt.Sprintf("pipeline_build_events_%d", defaultPipeline.ID())

				var exists bool
				err := dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = '" + tableName + "_build_id')").Scan(&exists)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue(), "didn't create '" + tableName + "_build_id' index")

				err = dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = '" + tableName + "_build_id_event_id')").Scan(&exists)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue(), "didn't create '" + tableName + "_build_id_event_id' index")
			})

			It("creates an event id sequence", func() {
				seqName := fmt.Sprintf("build_event_id_seq_%d", build.ID())
				var exists bool
				err := dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relkind = 'S' AND relname = '" + seqName + "')").Scan(&exists)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue(), "didn't create '" + seqName + "' sequence")
			})

			It("doesn't error when the pipeline build events table exists", func() {
				otherBuild, err := defaultPipeline.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())

				err = eventStore.Initialize(context.TODO(), otherBuild)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when the build is a one-off build", func() {
			BeforeEach(func() {
				var err error
				build, err = defaultTeam.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())
			})

			It("creates a team build events table", func() {
				tableName := fmt.Sprintf("team_build_events_%d", defaultTeam.ID())

				var exists bool
				err := dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = '" + tableName + "')").Scan(&exists)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue(), "didn't create '" + tableName + "' table")
			})
		})
	})

	Describe("Finalize", func() {
		var build db.Build

		BeforeEach(func() {
			var err error
			build, err = defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())
		})

		It("drops the event id sequence", func() {
			err := eventStore.Initialize(context.TODO(), build)
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Finalize(context.TODO(), build)
			Expect(err).ToNot(HaveOccurred())

			seqName := fmt.Sprintf("build_event_id_seq_%d", build.ID())
			var exists bool
			err = dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relkind = 'S' AND relname = '" + seqName + "')").Scan(&exists)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse(), "didn't delete '" + seqName + "' sequence")
		})
	})

	Describe("Put/Get", func() {
		var build db.Build

		BeforeEach(func() {
			var err error
			build, err = defaultPipeline.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Initialize(context.TODO(), build)
			Expect(err).ToNot(HaveOccurred())
		})

		It("storing and retrieving single events", func() {
			err := eventStore.Put(context.TODO(), build, []atc.Event{event.Start{}})
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Put(context.TODO(), build, []atc.Event{
				event.Log{Payload: "hello"},
				event.Log{Payload: "world"},
			})
			Expect(err).ToNot(HaveOccurred())

			var cursor db.Key
			events, err := eventStore.Get(context.TODO(), build, 3, &cursor)
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(Equal([]event.Envelope{
				envelope(event.Start{}),
				envelope(event.Log{Payload: "hello"}),
				envelope(event.Log{Payload: "world"}),
			}))
		})

		It("supports pagination", func() {
			err := eventStore.Put(context.TODO(), build, []atc.Event{
				event.Log{Payload: "A"},
				event.Log{Payload: "B"},
				event.Log{Payload: "C"},
				event.Log{Payload: "D"},
				event.Log{Payload: "E"},
			})
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Finalize(context.TODO(), build)
			Expect(err).ToNot(HaveOccurred())

			var cursor db.Key
			events, err := eventStore.Get(context.TODO(), build, 2, &cursor)
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(Equal([]event.Envelope{
				envelope(event.Log{Payload: "A"}),
				envelope(event.Log{Payload: "B"}),
			}))

			events, err = eventStore.Get(context.TODO(), build, 2, &cursor)
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(Equal([]event.Envelope{
				envelope(event.Log{Payload: "C"}),
				envelope(event.Log{Payload: "D"}),
			}))

			events, err = eventStore.Get(context.TODO(), build, 2, &cursor)
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(Equal([]event.Envelope{
				envelope(event.Log{Payload: "E"}),
			}))
		})
	})

	Describe("Delete", func() {
		var build1 db.Build
		var build2 db.Build

		BeforeEach(func() {
			var err error
			build1, err = defaultPipeline.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			build2, err = defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Setup(context.TODO())
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Initialize(context.TODO(), build1)
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Initialize(context.TODO(), build2)
			Expect(err).ToNot(HaveOccurred())
		})

		It("deletes all events from the provided builds", func() {
			err := eventStore.Put(context.TODO(), build1, []atc.Event{event.Start{}})
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Put(context.TODO(), build2, []atc.Event{
				event.Log{Payload: "hello"},
				event.Log{Payload: "world"},
			})
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Delete(context.TODO(), []db.Build{build1, build2})
			Expect(err).ToNot(HaveOccurred())

			var cursor db.Key
			events, err := eventStore.Get(context.TODO(), build1, 100, &cursor)
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(BeEmpty())
			Expect(cursor).To(BeNil())

			events, err = eventStore.Get(context.TODO(), build2, 100, &cursor)
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(BeEmpty())
		})
	})

	Describe("DeletePipeline", func() {
		var build db.Build

		BeforeEach(func() {
			var err error
			build, err = defaultPipeline.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Setup(context.TODO())
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Initialize(context.TODO(), build)
			Expect(err).ToNot(HaveOccurred())
		})

		It("drops the pipeline build events table", func() {
			err := eventStore.DeletePipeline(context.TODO(), defaultPipeline)
			Expect(err).ToNot(HaveOccurred())

			tableName := fmt.Sprintf("pipeline_build_events_%d", defaultPipeline.ID())
			var exists bool
			err = dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = '" + tableName + "')").Scan(&exists)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse(), "didn't drop '" + tableName + "' table")
		})
	})

	Describe("DeleteTeam", func() {
		var pipelineBuild db.Build
		var teamBuild db.Build

		BeforeEach(func() {
			var err error
			pipelineBuild, err = defaultPipeline.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			teamBuild, err = defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Setup(context.TODO())
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Initialize(context.TODO(), pipelineBuild)
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Initialize(context.TODO(), teamBuild)
			Expect(err).ToNot(HaveOccurred())
		})

		It("drops the team build events table", func() {
			err := eventStore.DeleteTeam(context.TODO(), defaultTeam)
			Expect(err).ToNot(HaveOccurred())

			tableName := fmt.Sprintf("team_build_events_%d", defaultTeam.ID())
			var exists bool
			err = dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = '" + tableName + "')").Scan(&exists)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse(), "didn't drop '" + tableName + "' table")
		})

		It("drops the pipeline build events table for pipelines in that team", func() {
			err := eventStore.DeleteTeam(context.TODO(), defaultTeam)
			Expect(err).ToNot(HaveOccurred())

			tableName := fmt.Sprintf("pipeline_build_events_%d", defaultPipeline.ID())
			var exists bool
			err = dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = '" + tableName + "')").Scan(&exists)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse(), "didn't drop '" + tableName + "' table")
		})
	})
})
