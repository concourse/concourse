package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/events"
	"github.com/concourse/concourse/atc/events/postgres"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Store", func() {
	var (
		eventStore events.Store

		fakePipeline      *dbfakes.FakePipeline
		fakeTeam          *dbfakes.FakeTeam
		fakePipelineBuild *dbfakes.FakeBuild
		fakeTeamBuild     *dbfakes.FakeBuild

		ctx context.Context
	)

	BeforeEach(func() {
		eventStore = &postgres.Store{
			Conn: dbConn,
		}

		fakePipeline = new(dbfakes.FakePipeline)
		fakePipeline.IDReturns(2)
		fakeTeam = new(dbfakes.FakeTeam)
		fakeTeam.IDReturns(3)

		fakePipelineBuild = new(dbfakes.FakeBuild)
		fakePipelineBuild.IDReturns(1)
		fakePipelineBuild.PipelineIDReturns(fakePipeline.ID())

		fakeTeamBuild = new(dbfakes.FakeBuild)
		fakeTeamBuild.IDReturns(2)
		fakeTeamBuild.TeamIDReturns(fakeTeam.ID())

		ctx = context.Background()
	})

	BeforeEach(func() {
		err := eventStore.Setup(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Setup", func() {
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
			err := eventStore.Setup(ctx)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Initialize", func() {
		var build db.Build

		JustBeforeEach(func() {
			err := eventStore.Initialize(ctx, build)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the build is a pipeline build", func() {
			BeforeEach(func() {
				build = fakePipelineBuild
			})

			It("creates a pipeline build events table", func() {
				tableName := fmt.Sprintf("pipeline_build_events_%d", fakePipeline.ID())

				var exists bool
				err := dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = '" + tableName + "')").Scan(&exists)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue(), "didn't create '"+tableName+"' table")
			})

			It("creates the required indexes", func() {
				tableName := fmt.Sprintf("pipeline_build_events_%d", fakePipeline.ID())

				var exists bool
				err := dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = '" + tableName + "_build_id')").Scan(&exists)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue(), "didn't create '"+tableName+"_build_id' index")

				err = dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = '" + tableName + "_build_id_event_id')").Scan(&exists)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue(), "didn't create '"+tableName+"_build_id_event_id' index")
			})

			It("creates an event id sequence", func() {
				seqName := fmt.Sprintf("build_event_id_seq_%d", fakePipelineBuild.ID())
				var exists bool
				err := dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relkind = 'S' AND relname = '" + seqName + "')").Scan(&exists)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue(), "didn't create '"+seqName+"' sequence")
			})

			It("doesn't error when the pipeline build events table exists", func() {
				otherBuild := new(dbfakes.FakeBuild)
				otherBuild.PipelineIDReturns(fakePipeline.ID())

				err := eventStore.Initialize(ctx, otherBuild)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when the build is a one-off build", func() {
			BeforeEach(func() {
				build = fakeTeamBuild
			})

			It("creates a team build events table", func() {
				tableName := fmt.Sprintf("team_build_events_%d", fakeTeam.ID())

				var exists bool
				err := dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = '" + tableName + "')").Scan(&exists)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue(), "didn't create '"+tableName+"' table")
			})
		})
	})

	Describe("Finalize", func() {
		It("drops the event id sequence", func() {
			err := eventStore.Initialize(ctx, fakePipelineBuild)
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Finalize(ctx, fakePipelineBuild)
			Expect(err).ToNot(HaveOccurred())

			seqName := fmt.Sprintf("build_event_id_seq_%d", fakePipelineBuild.ID())
			var exists bool
			err = dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relkind = 'S' AND relname = '" + seqName + "')").Scan(&exists)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse(), "didn't delete '"+seqName+"' sequence")
		})
	})

	Describe("Put/Get", func() {
		BeforeEach(func() {
			err := eventStore.Initialize(ctx, fakePipelineBuild)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Put returns the next event_id as the cursor", func() {
			cursor, err := eventStore.Put(ctx, fakePipelineBuild, []atc.Event{event.Start{}, event.Log{}})
			Expect(err).ToNot(HaveOccurred())
			Expect(cursor).To(Equal(postgres.EventID(2)))

			cursor, err = eventStore.Put(ctx, fakePipelineBuild, []atc.Event{event.Finish{}})
			Expect(err).ToNot(HaveOccurred())
			Expect(cursor).To(Equal(postgres.EventID(3)))
		})

		It("storing and retrieving single events", func() {
			_, err := eventStore.Put(ctx, fakePipelineBuild, []atc.Event{event.Start{}})
			Expect(err).ToNot(HaveOccurred())

			_, err = eventStore.Put(ctx, fakePipelineBuild, []atc.Event{
				event.Log{Payload: "hello"},
				event.Log{Payload: "world"},
			})
			Expect(err).ToNot(HaveOccurred())

			var cursor db.EventKey
			events, err := eventStore.Get(ctx, fakePipelineBuild, 3, &cursor)
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(Equal([]event.Envelope{
				envelope(event.Start{}),
				envelope(event.Log{Payload: "hello"}),
				envelope(event.Log{Payload: "world"}),
			}))
		})

		It("supports pagination", func() {
			_, err := eventStore.Put(ctx, fakePipelineBuild, []atc.Event{
				event.Log{Payload: "A"},
				event.Log{Payload: "B"},
				event.Log{Payload: "C"},
				event.Log{Payload: "D"},
				event.Log{Payload: "E"},
			})
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Finalize(ctx, fakePipelineBuild)
			Expect(err).ToNot(HaveOccurred())

			var cursor db.EventKey
			events, err := eventStore.Get(ctx, fakePipelineBuild, 2, &cursor)
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(Equal([]event.Envelope{
				envelope(event.Log{Payload: "A"}),
				envelope(event.Log{Payload: "B"}),
			}))

			events, err = eventStore.Get(ctx, fakePipelineBuild, 2, &cursor)
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(Equal([]event.Envelope{
				envelope(event.Log{Payload: "C"}),
				envelope(event.Log{Payload: "D"}),
			}))

			events, err = eventStore.Get(ctx, fakePipelineBuild, 2, &cursor)
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(Equal([]event.Envelope{
				envelope(event.Log{Payload: "E"}),
			}))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			err := eventStore.Initialize(ctx, fakePipelineBuild)
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Initialize(ctx, fakeTeamBuild)
			Expect(err).ToNot(HaveOccurred())
		})

		It("deletes all events from the provided builds", func() {
			_, err := eventStore.Put(ctx, fakePipelineBuild, []atc.Event{event.Start{}})
			Expect(err).ToNot(HaveOccurred())

			_, err = eventStore.Put(ctx, fakeTeamBuild, []atc.Event{
				event.Log{Payload: "hello"},
				event.Log{Payload: "world"},
			})
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Delete(ctx, []db.Build{fakePipelineBuild, fakeTeamBuild})
			Expect(err).ToNot(HaveOccurred())

			var cursor db.EventKey
			events, err := eventStore.Get(ctx, fakePipelineBuild, 100, &cursor)
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(BeEmpty())
			Expect(cursor).To(BeNil())

			events, err = eventStore.Get(ctx, fakeTeamBuild, 100, &cursor)
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(BeEmpty())
		})
	})

	Describe("DeletePipeline", func() {
		BeforeEach(func() {
			err := eventStore.Initialize(ctx, fakePipelineBuild)
			Expect(err).ToNot(HaveOccurred())
		})

		It("drops the pipeline build events table", func() {
			err := eventStore.DeletePipeline(ctx, fakePipeline)
			Expect(err).ToNot(HaveOccurred())

			tableName := fmt.Sprintf("pipeline_build_events_%d", fakePipeline.ID())
			var exists bool
			err = dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = '" + tableName + "')").Scan(&exists)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse(), "didn't drop '"+tableName+"' table")
		})
	})

	Describe("DeleteTeam", func() {
		BeforeEach(func() {
			err := eventStore.Initialize(ctx, fakePipelineBuild)
			Expect(err).ToNot(HaveOccurred())

			err = eventStore.Initialize(ctx, fakeTeamBuild)
			Expect(err).ToNot(HaveOccurred())
		})

		It("drops the team build events table", func() {
			err := eventStore.DeleteTeam(ctx, fakeTeam)
			Expect(err).ToNot(HaveOccurred())

			tableName := fmt.Sprintf("team_build_events_%d", fakeTeam.ID())
			var exists bool
			err = dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = '" + tableName + "')").Scan(&exists)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse(), "didn't drop '"+tableName+"' table")
		})

		It("drops the pipeline build events table for pipelines in that team", func() {
			err := eventStore.DeleteTeam(ctx, fakeTeam)
			Expect(err).ToNot(HaveOccurred())

			tableName := "pipeline_build_events_1"
			var exists bool
			err = dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = '" + tableName + "')").Scan(&exists)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse(), "didn't drop '"+tableName+"' table")
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
