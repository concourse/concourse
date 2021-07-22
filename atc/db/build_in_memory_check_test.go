package db_test

import (
	"code.cloudfoundry.org/lager"
	"context"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/util"
	"github.com/concourse/concourse/tracing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Build", func() {
	var (
		build db.Build
		ctx   context.Context
		plan  atc.Plan
	)

	BeforeEach(func() {
		plan = atc.Plan{
			ID: "1234",
			Check: &atc.CheckPlan{
				Name:   "some-resource-check",
				Type:   "some-base-resource-type",
				Source: atc.Source{"some-key": "some-value"},
			},
		}
	})

	Describe("Running in-memory-build", func() {
		BeforeEach(func() {
			ctx = context.Background()
		})

		BeforeEach(func() {
			var err error
			build, err = defaultResource.CreateInMemoryBuild(ctx, plan, util.NewSequenceGenerator(1))
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates a build", func() {
			Expect(build).NotTo(BeNil())

			Expect(build.ID()).To(Equal(0))
			Expect(build.Name()).To(Equal(db.CheckBuildName))
			Expect(build.TeamID()).To(Equal(defaultResource.TeamID()))
			Expect(build.TeamName()).To(Equal(defaultResource.TeamName()))
			Expect(build.PipelineID()).To(Equal(defaultResource.PipelineID()))
			Expect(build.PipelineName()).To(Equal(defaultResource.PipelineName()))
			Expect(build.JobID()).To(Equal(0))
			Expect(build.JobName()).To(BeEmpty())
			Expect(build.ResourceID()).To(Equal(defaultResource.ID()))
			Expect(build.ResourceName()).To(Equal(defaultResource.Name()))
			Expect(build.ResourceTypeID()).To(Equal(0))
			Expect(build.Schema()).To(Equal("exec.v2"))

			Expect(build.IsRunning()).To(BeTrue())
			Expect(build.IsManuallyTriggered()).To(BeFalse())

			Expect(build.HasPlan()).To(BeTrue())
			Expect(build.PrivatePlan()).To(Equal(plan))
			Expect(*build.PublicPlan()).To(Equal(*plan.Public()))
		})

		It("LagerData", func() {
			Expect(build.LagerData()).To(Equal(lager.Data{
				"team":       defaultResource.TeamName(),
				"pipeline":   defaultResource.PipelineName(),
				"preBuildId": 1,
				"resource":   defaultResource.Name(),
			}))
		})

		It("TracingAttrs", func() {
			Expect(build.TracingAttrs()).To(Equal(tracing.Attrs{
				"team":       defaultResource.TeamName(),
				"pipeline":   defaultResource.PipelineName(),
				"preBuildId": "1",
				"resource":   defaultResource.Name(),
			}))
		})

		It("SpanContext", func() {
			Expect(build.SpanContext()).To(Equal(db.NewSpanContext(ctx)))
		})

		It("Event should fail", func() {
			_, err := build.Events(0)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("no-build-event-yet"))
		})

		It("should panic before build starts", func() {
			Expect(func() { build.ContainerOwner("some-plan") }).Should(PanicWith(Equal("in-memory-build-not-running-yet")))
		})

		Context("start to run", func() {
			BeforeEach(func() {
				err := build.SaveEvent(event.Initialize{
					Origin: event.Origin{
						ID: event.OriginID(plan.ID),
					},
					Time: time.Now().Unix(),
				})
				Expect(err).ToNot(HaveOccurred())

				err = build.SaveEvent(event.Start{
					Origin: event.Origin{
						ID: event.OriginID(plan.ID),
					},
					Time: time.Now().Unix(),
				})
				Expect(err).ToNot(HaveOccurred())
			})

			It("Event should fail", func() {
				_, err := build.Events(0)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("no-build-event-yet"))
			})

			Context("OnCheckBuildStart", func() {
				var buildId int

				BeforeEach(func() {
					err := build.OnCheckBuildStart()
					Expect(err).ToNot(HaveOccurred())

					buildId = build.ID()
				})

				It("get a build id", func() {
					Expect(buildId).To(BeNumerically(">", 0))
				})

				It("LagerData", func() {
					Expect(build.LagerData()).To(Equal(lager.Data{
						"build":      buildId,
						"team":       defaultResource.TeamName(),
						"pipeline":   defaultResource.PipelineName(),
						"preBuildId": 1,
						"resource":   defaultResource.Name(),
					}))
				})

				It("TracingAttrs", func() {
					Expect(build.TracingAttrs()).To(Equal(tracing.Attrs{
						"build":      fmt.Sprintf("%d", buildId),
						"team":       defaultResource.TeamName(),
						"pipeline":   defaultResource.PipelineName(),
						"preBuildId": "1",
						"resource":   defaultResource.Name(),
					}))
				})

				It("saved build events", func() {
					eventSource, err := build.Events(0)
					Expect(err).ToNot(HaveOccurred())

					ev0, err := eventSource.Next()
					Expect(err).ToNot(HaveOccurred())
					Expect(ev0.Event).To(Equal(event.EventTypeStatus))
					Expect(ev0.EventID).To(Equal("0"))

					ev1, err := eventSource.Next()
					Expect(err).ToNot(HaveOccurred())
					Expect(ev1.Event).To(Equal(event.EventTypeInitialize))
					Expect(ev1.EventID).To(Equal("1"))

					ev2, err := eventSource.Next()
					Expect(err).ToNot(HaveOccurred())
					Expect(ev2.Event).To(Equal(event.EventTypeStart))
					Expect(ev2.EventID).To(Equal("2"))

					err = eventSource.Close()
					Expect(err).ToNot(HaveOccurred())
				})

				It("save a new event", func() {
					err := build.SaveEvent(event.Log{
						Origin: event.Origin{
							ID: event.OriginID(plan.ID),
						},
						Time:    time.Now().Unix(),
						Payload: "some-log-line",
					})
					Expect(err).ToNot(HaveOccurred())

					eventSource, err := build.Events(3)
					Expect(err).ToNot(HaveOccurred())

					ev, err := eventSource.Next()
					Expect(err).ToNot(HaveOccurred())
					Expect(ev.Event).To(Equal(event.EventTypeLog))
					Expect(ev.EventID).To(Equal("3"))
				})

				It("ResourceCacheUser", func() {
					Expect(build.ResourceCacheUser()).To(Equal(db.NoUser()))
				})

				It("ContainerOwner", func() {
					Expect(build.ContainerOwner("some-plan")).To(Equal(
						db.NewInMemoryCheckBuildContainerOwner(buildId, "some-plan", defaultResource.TeamID())))
				})

				It("RunStateID", func() {
					Expect(build.RunStateID()).To(Equal(fmt.Sprintf("in-memory-check-build:%d", buildId)))
				})

				Context("AcquireTrackingLock", func() {
					var l lock.Lock
					BeforeEach(func() {
						var acquired bool
						var err error
						l, acquired, err = build.AcquireTrackingLock(logger, time.Second)
						Expect(err).ToNot(HaveOccurred())
						Expect(acquired).To(BeTrue())
					})

					It("cannot acquire the lock when it's acquired already", func() {
						_, acquired, err := build.AcquireTrackingLock(logger, time.Second)
						Expect(err).ToNot(HaveOccurred())
						Expect(acquired).To(BeFalse())
					})

					AfterEach(func() {
						err := l.Release()
						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("Finish", func() {
					BeforeEach(func() {
						result, err := psql.Insert("containers").
							Columns("handle", "plan_id", "pipeline_id", "resource_id", "worker_name", "team_id", "in_memory_check_build_id").
							Values("some-handle-1234567890", "some-plan", defaultResource.PipelineID(), defaultResource.ID(), defaultWorker.Name(), defaultResource.TeamID(), buildId).
							RunWith(dbConn).
							Exec()
						Expect(err).ToNot(HaveOccurred())
						Expect(result.RowsAffected()).To(Equal(int64(1)))

						err = build.Finish(db.BuildStatusSucceeded)
						Expect(err).ToNot(HaveOccurred())
					})

					It("save build status", func() {
						eventSource, err := build.Events(3)
						Expect(err).ToNot(HaveOccurred())

						ev, err := eventSource.Next()
						Expect(err).ToNot(HaveOccurred())
						Expect(ev.Event).To(Equal(event.EventTypeStatus))
						Expect(ev.EventID).To(Equal("3"))
					})

					It("cleanup containers", func() {
						rows, err := psql.Select("id").
							From("containers").
							Where(sq.Eq{"in_memory_check_build_id": buildId}).
							RunWith(dbConn).
							Query()
						Expect(err).ToNot(HaveOccurred())
						Expect(rows.Next()).To(BeFalse())
						Expect(rows.Close()).ToNot(HaveOccurred())
					})
				})
			})
		})
	})

	Describe("Existing in-memory build", func() {
		var build db.Build

		BeforeEach(func() {
			resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(defaultResource.Type(), defaultResource.Source(), atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			scope, err := resourceConfig.FindOrCreateScope(defaultResource)
			Expect(err).ToNot(HaveOccurred())

			found, err := scope.UpdateLastCheckStartTime(1999, plan.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = defaultResource.SetResourceConfigScope(scope)
			Expect(err).ToNot(HaveOccurred())

			build, found, err = buildFactory.Build(1999)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("find a build", func() {
			Expect(build).NotTo(BeNil())
			Expect(build.ID()).To(Equal(1999))
			Expect(build.Name()).To(Equal(db.CheckBuildName))
			Expect(build.TeamID()).To(Equal(defaultResource.TeamID()))
			Expect(build.TeamName()).To(Equal(defaultResource.TeamName()))
			Expect(build.PipelineID()).To(Equal(defaultResource.PipelineID()))
			Expect(build.PipelineName()).To(Equal(defaultResource.PipelineName()))
			Expect(build.JobID()).To(Equal(0))
			Expect(build.JobName()).To(BeEmpty())
			Expect(build.ResourceID()).To(Equal(defaultResource.ID()))
			Expect(build.ResourceName()).To(Equal(defaultResource.Name()))
			Expect(build.ResourceTypeID()).To(Equal(0))
			Expect(build.Schema()).To(Equal("exec.v2"))

			Expect(build.IsRunning()).To(BeFalse())
			Expect(build.IsManuallyTriggered()).To(BeFalse())

			Expect(build.HasPlan()).To(BeTrue())
			Expect(*build.PublicPlan()).To(Equal(*plan.Public()))
		})

		It("LagerData", func() {
			Expect(build.LagerData()).To(Equal(lager.Data{
				"team":     defaultResource.TeamName(),
				"pipeline": defaultResource.PipelineName(),
				"build":    1999,
				"resource": defaultResource.Name(),
			}))
		})

		It("TracingAttrs", func() {
			Expect(build.TracingAttrs()).To(Equal(tracing.Attrs{
				"team":     defaultResource.TeamName(),
				"pipeline": defaultResource.PipelineName(),
				"build":    "1999",
				"resource": defaultResource.Name(),
			}))
		})

		It("Events", func() {
			_, err := build.Events(0)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should panic", func() {
			Expect(func() { build.OnCheckBuildStart() }).Should(PanicWith(Equal("not-implemented")))
			Expect(func() { build.PrivatePlan() }).Should(PanicWith(Equal("not-implemented")))
			Expect(func() { build.Finish(db.BuildStatusSucceeded) }).Should(PanicWith(Equal("not-implemented")))
			Expect(func() { build.AcquireTrackingLock(logger, time.Minute) }).Should(PanicWith(Equal("not-implemented")))
		})
	})
})
