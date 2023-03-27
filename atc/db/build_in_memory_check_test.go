package db_test

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/util"
	"github.com/concourse/concourse/tracing"

	. "github.com/onsi/ginkgo/v2"
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
				Name:   defaultResource.Name(),
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
			Expect(build.Status()).To(Equal(db.BuildStatusPending))

			Expect(build.IsRunning()).To(BeTrue())
			Expect(build.IsManuallyTriggered()).To(BeFalse())

			Expect(build.HasPlan()).To(BeTrue())
			Expect(build.PrivatePlan()).To(Equal(plan))
			Expect(*build.PublicPlan()).To(Equal(*plan.Public()))
		})

		It("LagerData", func() {
			Expect(build.LagerData()).To(Equal(lager.Data{
				"team":         defaultResource.TeamName(),
				"pipeline":     defaultResource.PipelineName(),
				"pre_build_id": 1,
				"resource":     defaultResource.Name(),
				"build":        "check",
			}))
		})

		It("TracingAttrs", func() {
			Expect(build.TracingAttrs()).To(Equal(tracing.Attrs{
				"team":         defaultResource.TeamName(),
				"pipeline":     defaultResource.PipelineName(),
				"pre_build_id": "1",
				"resource":     defaultResource.Name(),
				"build":        "check",
			}))
		})

		It("SpanContext", func() {
			Expect(build.SpanContext()).To(Equal(db.NewSpanContext(ctx)))
		})

		It("Event should fail", func() {
			_, err := build.Events(0)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("no build event"))
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
				Expect(err.Error()).To(Equal("no build event"))
			})

			Context("BeforeCheckBuildStart", func() {
				Context("Finish", func() {
					Context("when build finishes with status succeed", func() {
						BeforeEach(func() {
							err := build.Finish(db.BuildStatusSucceeded)
							Expect(err).ToNot(HaveOccurred())
						})

						It("does not init DB", func() {
							Expect(build.ID()).To(Equal(0))
						})
					})

					Context("when build finishes with status errored", func() {
						BeforeEach(func() {
							err := build.Finish(db.BuildStatusErrored)
							Expect(err).ToNot(HaveOccurred())
						})

						It("update in-memory build status for resource", func() {
							reloaded, err := defaultResource.Reload()
							Expect(reloaded).To(BeTrue())
							Expect(err).ToNot(HaveOccurred())
							Expect(defaultResource.BuildSummary().Status).To(Equal(atc.StatusErrored))
						})

						It("save build status", func() {
							eventSource, err := build.Events(3)
							Expect(err).ToNot(HaveOccurred())

							ev, err := eventSource.Next()
							Expect(err).ToNot(HaveOccurred())
							Expect(ev.Event).To(Equal(event.EventTypeStatus))
							Expect(ev.EventID).To(Equal("3"))
							Expect(string(*ev.Data)).To(ContainSubstring("errored"))
						})
					})
				})
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

				It("update in-memory build status for resource", func() {
					reloaded, err := defaultResource.Reload()
					Expect(reloaded).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					Expect(defaultResource.BuildSummary().Status).To(Equal(atc.StatusStarted))
				})

				It("LagerData", func() {
					Expect(build.LagerData()).To(Equal(lager.Data{
						"build_id":     buildId,
						"team":         defaultResource.TeamName(),
						"pipeline":     defaultResource.PipelineName(),
						"pre_build_id": 1,
						"resource":     defaultResource.Name(),
						"build":        "check",
					}))
				})

				It("TracingAttrs", func() {
					Expect(build.TracingAttrs()).To(Equal(tracing.Attrs{
						"build_id":     fmt.Sprintf("%d", buildId),
						"team":         defaultResource.TeamName(),
						"pipeline":     defaultResource.PipelineName(),
						"pre_build_id": "1",
						"resource":     defaultResource.Name(),
						"build":        "check",
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
					Expect(build.ResourceCacheUser()).To(Equal(db.ForInMemoryBuild(1, build.CreateTime())))
				})

				It("ContainerOwner", func() {
					Expect(build.ContainerOwner("some-plan")).To(Equal(
						db.NewInMemoryCheckBuildContainerOwner(buildId, build.CreateTime(), "some-plan", defaultResource.TeamID())))
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
						err := build.Finish(db.BuildStatusSucceeded)
						Expect(err).ToNot(HaveOccurred())
					})

					It("update in-memory build status for resource", func() {
						reloaded, err := defaultResource.Reload()
						Expect(reloaded).To(BeTrue())
						Expect(err).ToNot(HaveOccurred())

						Expect(defaultResource.BuildSummary().Status).To(Equal(atc.StatusSucceeded))
					})

					It("save build status", func() {
						eventSource, err := build.Events(3)
						Expect(err).ToNot(HaveOccurred())

						ev, err := eventSource.Next()
						Expect(err).ToNot(HaveOccurred())
						Expect(ev.Event).To(Equal(event.EventTypeStatus))
						Expect(ev.EventID).To(Equal("3"))
					})
				})
			})
		})
	})

	Describe("in-memory build for api", func() {
		var build db.BuildForAPI

		BeforeEach(func() {
			resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(defaultResource.Type(), defaultResource.Source(), nil)
			Expect(err).ToNot(HaveOccurred())

			scope, err := resourceConfig.FindOrCreateScope(intptr(defaultResource.ID()))
			Expect(err).ToNot(HaveOccurred())

			found, err := scope.UpdateLastCheckStartTime(1999, plan.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = defaultResource.SetResourceConfigScope(scope)
			Expect(err).ToNot(HaveOccurred())

			build, found, err = buildFactory.BuildForAPI(1999)
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
			Expect(build.Schema()).To(Equal("exec.v2"))

			Expect(build.IsRunning()).To(BeFalse())

			Expect(build.HasPlan()).To(BeTrue())
			Expect(*build.PublicPlan()).To(Equal(*plan.Public()))
		})

		It("LagerData", func() {
			Expect(build.LagerData()).To(Equal(lager.Data{
				"team":     defaultResource.TeamName(),
				"pipeline": defaultResource.PipelineName(),
				"build_id": 1999,
				"resource": defaultResource.Name(),
				"build":    "check",
			}))
		})

		It("Events", func() {
			_, err := build.Events(0)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
