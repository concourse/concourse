package scheduler_test

import (
	"context"
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/schedulerfakes"
	"github.com/concourse/concourse/tracing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/oteltest"
	"go.opentelemetry.io/otel/trace"
)

var _ = Describe("Scheduler", func() {
	var (
		fakeAlgorithm    *schedulerfakes.FakeAlgorithm
		fakeBuildStarter *schedulerfakes.FakeBuildStarter

		scheduler *Scheduler

		disaster error
		ctx      context.Context
	)

	BeforeEach(func() {
		fakeAlgorithm = new(schedulerfakes.FakeAlgorithm)
		fakeBuildStarter = new(schedulerfakes.FakeBuildStarter)

		scheduler = &Scheduler{
			Algorithm:    fakeAlgorithm,
			BuildStarter: fakeBuildStarter,
		}

		disaster = errors.New("bad thing")
	})

	Describe("Schedule", func() {
		var (
			fakePipeline *dbfakes.FakePipeline
			fakeJob      *dbfakes.FakeJob
			scheduleErr  error
		)

		BeforeEach(func() {
			fakeJob = new(dbfakes.FakeJob)
			fakePipeline = new(dbfakes.FakePipeline)
			fakePipeline.NameReturns("fake-pipeline")
			ctx = context.Background()
		})

		JustBeforeEach(func() {
			var waiter interface{ Wait() }

			_, scheduleErr = scheduler.Schedule(
				ctx,
				lagertest.NewTestLogger("test"),
				db.SchedulerJob{
					Job: fakeJob,
					Resources: db.SchedulerResources{
						{
							Name: "some-resource",
						},
					},
				},
			)
			if waiter != nil {
				waiter.Wait()
			}
		})

		Context("when the job has no inputs", func() {
			BeforeEach(func() {
				fakeJob.NameReturns("some-job-1")

				fakeJob.AlgorithmInputsReturns(nil, nil)
			})

			Context("when computing the inputs fails", func() {
				BeforeEach(func() {
					fakeAlgorithm.ComputeReturns(nil, false, false, disaster)
				})

				It("returns the error", func() {
					Expect(scheduleErr).To(Equal(fmt.Errorf("compute inputs: %w", disaster)))
				})
			})

			Context("when computing the inputs succeeds", func() {
				var expectedInputMapping db.InputMapping

				BeforeEach(func() {
					expectedInputMapping = map[string]db.InputResult{
						"input-1": db.InputResult{
							Input: &db.AlgorithmInput{
								AlgorithmVersion: db.AlgorithmVersion{
									ResourceID: 1,
									Version:    db.ResourceVersion("1"),
								},
								FirstOccurrence: true,
							},
						},
					}

					fakeAlgorithm.ComputeReturns(expectedInputMapping, true, false, nil)
				})

				It("computed the inputs", func() {
					Expect(fakeAlgorithm.ComputeCallCount()).To(Equal(1))
					_, actualJob, actualInputs := fakeAlgorithm.ComputeArgsForCall(0)
					Expect(actualJob.Name()).To(Equal(fakeJob.Name()))
					Expect(actualInputs).To(BeNil())
				})

				Context("when the algorithm can run again", func() {
					BeforeEach(func() {
						fakeAlgorithm.ComputeReturns(expectedInputMapping, true, true, nil)
					})

					It("requests schedule on the pipeline", func() {
						Expect(fakeJob.RequestScheduleCallCount()).To(Equal(1))
					})
				})

				Context("when the algorithm can not compute a next set of inputs", func() {
					BeforeEach(func() {
						fakeAlgorithm.ComputeReturns(expectedInputMapping, true, false, nil)
					})

					It("does not request schedule on the pipeline", func() {
						Expect(fakeJob.RequestScheduleCallCount()).To(Equal(0))
					})
				})

				Context("when saving the next input mapping fails", func() {
					BeforeEach(func() {
						fakeJob.SaveNextInputMappingReturns(disaster)
					})

					It("returns the error", func() {
						Expect(scheduleErr).To(Equal(fmt.Errorf("save next input mapping: %w", disaster)))
					})
				})

				Context("when saving the next input mapping succeeds", func() {
					BeforeEach(func() {
						fakeJob.SaveNextInputMappingReturns(nil)
					})

					It("saved the next input mapping", func() {
						Expect(fakeJob.SaveNextInputMappingCallCount()).To(Equal(1))
						actualInputMapping, resolved := fakeJob.SaveNextInputMappingArgsForCall(0)
						Expect(actualInputMapping).To(Equal(expectedInputMapping))
						Expect(resolved).To(BeTrue())
					})

					Context("when getting the full next build inputs fails", func() {
						BeforeEach(func() {
							fakeJob.GetFullNextBuildInputsReturns(nil, false, disaster)
						})

						It("returns the error", func() {
							Expect(scheduleErr).To(Equal(fmt.Errorf("get next build inputs: %w", disaster)))
						})
					})

					Context("when getting the full next build inputs succeeds", func() {
						BeforeEach(func() {
							fakeJob.GetFullNextBuildInputsReturns([]db.BuildInput{}, true, nil)
						})

						Context("when starting pending builds for job fails", func() {
							BeforeEach(func() {
								fakeBuildStarter.TryStartPendingBuildsForJobReturns(false, disaster)
							})

							It("returns the error", func() {
								Expect(scheduleErr).To(Equal(disaster))
							})

							It("started all pending builds", func() {
								Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
								_, actualJob, actualInputs := fakeBuildStarter.TryStartPendingBuildsForJobArgsForCall(0)
								Expect(actualJob.Name()).To(Equal(fakeJob.Name()))
								Expect(len(actualJob.Resources)).To(Equal(1))
								Expect(actualJob.Resources[0].Name).To(Equal("some-resource"))
								Expect(actualInputs).To(BeNil())
							})
						})

						Context("when starting all pending builds succeeds", func() {
							BeforeEach(func() {
								fakeBuildStarter.TryStartPendingBuildsForJobReturns(false, nil)
							})

							It("returns no error", func() {
								Expect(scheduleErr).NotTo(HaveOccurred())
							})

							It("didn't create a pending build", func() {
								//TODO: create a positive test case for this
								Expect(fakeJob.EnsurePendingBuildExistsCallCount()).To(BeZero())
							})
						})
					})
				})

				It("didn't mark the job as having new inputs", func() {
					Expect(fakeJob.SetHasNewInputsCallCount()).To(BeZero())
				})
			})
		})

		Context("when the job has one trigger: true input", func() {
			BeforeEach(func() {
				fakeJob.NameReturns("some-job")
				fakeJob.AlgorithmInputsReturns(db.InputConfigs{
					{Name: "a", Trigger: true},
					{Name: "b", Trigger: false},
				}, nil)

				fakeBuildStarter.TryStartPendingBuildsForJobReturns(false, nil)
				fakeJob.SaveNextInputMappingReturns(nil)
			})

			It("started the builds with the correct arguments", func() {
				Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
				_, actualJob, actualInputs := fakeBuildStarter.TryStartPendingBuildsForJobArgsForCall(0)
				Expect(actualJob.Name()).To(Equal(fakeJob.Name()))
				Expect(len(actualJob.Resources)).To(Equal(1))
				Expect(actualJob.Resources[0].Name).To(Equal("some-resource"))
				Expect(actualInputs).To(Equal(db.InputConfigs{
					{Name: "a", Trigger: true},
					{Name: "b", Trigger: false},
				}))
			})

			Context("when no input mapping is found", func() {
				BeforeEach(func() {
					fakeAlgorithm.ComputeReturns(db.InputMapping{}, false, false, nil)
				})

				It("starts all pending builds and returns no error", func() {
					Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
					Expect(scheduleErr).NotTo(HaveOccurred())
				})

				It("didn't create a pending build", func() {
					Expect(fakeJob.EnsurePendingBuildExistsCallCount()).To(BeZero())
				})

				It("didn't mark the job as having new inputs", func() {
					Expect(fakeJob.SetHasNewInputsCallCount()).To(BeZero())
				})
			})

			Context("when no first occurrence input has trigger: true", func() {
				BeforeEach(func() {
					fakeJob.GetFullNextBuildInputsReturns([]db.BuildInput{
						{
							Name:            "a",
							Version:         atc.Version{"ref": "v1"},
							ResourceID:      11,
							FirstOccurrence: false,
						},
						{
							Name:            "b",
							Version:         atc.Version{"ref": "v2"},
							ResourceID:      12,
							FirstOccurrence: true,
						},
					}, true, nil)
				})

				It("starts all pending builds and returns no error", func() {
					Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
					Expect(scheduleErr).NotTo(HaveOccurred())
				})

				It("didn't create a pending build", func() {
					Expect(fakeJob.EnsurePendingBuildExistsCallCount()).To(BeZero())
				})

				Context("when the job does not have new inputs since before", func() {
					BeforeEach(func() {
						fakeJob.HasNewInputsReturns(false)
					})

					Context("when marking job as having new input fails", func() {
						BeforeEach(func() {
							fakeJob.SetHasNewInputsReturns(disaster)
						})

						It("returns the error", func() {
							Expect(scheduleErr).To(Equal(fmt.Errorf("set has new inputs: %w", disaster)))
						})
					})

					Context("when marking job as having new input succeeds", func() {
						BeforeEach(func() {
							fakeJob.SetHasNewInputsReturns(nil)
						})

						It("did the needful", func() {
							Expect(fakeJob.SetHasNewInputsCallCount()).To(Equal(1))
							Expect(fakeJob.SetHasNewInputsArgsForCall(0)).To(Equal(true))
						})
					})
				})

				Context("when the job has new inputs since before", func() {
					BeforeEach(func() {
						fakeJob.HasNewInputsReturns(true)
					})

					It("doesn't mark the job as having new inputs", func() {
						Expect(fakeJob.SetHasNewInputsCallCount()).To(BeZero())
					})
				})
			})

			Context("when a first occurrence input has trigger: true", func() {
				BeforeEach(func() {
					fakeJob.GetFullNextBuildInputsReturns([]db.BuildInput{
						{
							Name:            "a",
							Version:         atc.Version{"ref": "v1"},
							ResourceID:      11,
							FirstOccurrence: true,
						},
						{
							Name:            "b",
							Version:         atc.Version{"ref": "v2"},
							ResourceID:      12,
							FirstOccurrence: false,
						},
					}, true, nil)
				})

				Context("when creating a pending build fails", func() {
					BeforeEach(func() {
						fakeJob.EnsurePendingBuildExistsReturns(disaster)
					})

					It("returns the error", func() {
						Expect(scheduleErr).To(Equal(fmt.Errorf("ensure pending build exists: %w", disaster)))
					})

					It("created a pending build for the right job", func() {
						Expect(fakeJob.EnsurePendingBuildExistsCallCount()).To(Equal(1))
					})
				})

				Context("when creating a pending build succeeds", func() {
					BeforeEach(func() {
						fakeJob.EnsurePendingBuildExistsReturns(nil)
					})

					It("starts all pending builds and returns no error", func() {
						Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
						Expect(scheduleErr).NotTo(HaveOccurred())
					})
				})
			})

			Context("when no first occurrence", func() {
				BeforeEach(func() {
					fakeJob.GetFullNextBuildInputsReturns([]db.BuildInput{
						{
							Name:            "a",
							Version:         atc.Version{"ref": "v1"},
							ResourceID:      11,
							FirstOccurrence: false,
						},
						{
							Name:            "b",
							Version:         atc.Version{"ref": "v2"},
							ResourceID:      12,
							FirstOccurrence: false,
						},
					}, true, nil)
				})

				Context("when job had new inputs", func() {
					BeforeEach(func() {
						fakeJob.HasNewInputsReturns(true)
					})

					It("marks the job as not having new inputs", func() {
						Expect(fakeJob.SetHasNewInputsCallCount()).To(Equal(1))
						Expect(fakeJob.SetHasNewInputsArgsForCall(0)).To(Equal(false))
					})
				})

				Context("when job did not have new inputs", func() {
					BeforeEach(func() {
						fakeJob.HasNewInputsReturns(false)
					})

					It("doesn't mark the the job as not having new inputs again", func() {
						Expect(fakeJob.SetHasNewInputsCallCount()).To(Equal(0))
					})
				})
			})
		})

		Context("when multiple first occurrence inputs have trigger: true and tracing is configured", func() {
			var inputCtx1, inputCtx2 context.Context

			BeforeEach(func() {
				fakeJob.NameReturns("some-job")
				fakeJob.AlgorithmInputsReturns(db.InputConfigs{
					{Name: "a", Trigger: true},
					{Name: "b", Trigger: false},
					{Name: "c", Trigger: true},
				}, nil)
				fakeBuildStarter.TryStartPendingBuildsForJobReturns(false, nil)
				fakeJob.SaveNextInputMappingReturns(nil)

				tracing.ConfigureTraceProvider(oteltest.NewTracerProvider())

				ctx, _ = tracing.StartSpan(context.Background(), "scheduler.Run", nil)
				inputCtx1, _ = tracing.StartSpan(context.Background(), "checker.Run", nil)
				inputCtx2, _ = tracing.StartSpan(context.Background(), "checker.Run", nil)
				fakeJob.GetFullNextBuildInputsReturns([]db.BuildInput{
					{
						Name:            "a",
						Version:         atc.Version{"ref": "v1"},
						ResourceID:      11,
						FirstOccurrence: true,
						Context:         db.NewSpanContext(inputCtx1),
					},
					{
						Name:            "b",
						Version:         atc.Version{"ref": "v2"},
						ResourceID:      12,
						FirstOccurrence: false,
					},
					{
						Name:            "c",
						Version:         atc.Version{"ref": "v3"},
						ResourceID:      13,
						FirstOccurrence: true,
						Context:         db.NewSpanContext(inputCtx2),
					},
				}, true, nil)
			})

			AfterEach(func() {
				tracing.Configured = false
			})

			It("starts a linked span", func() {
				pendingBuildCtx := fakeJob.EnsurePendingBuildExistsArgsForCall(0)
				span := tracing.FromContext(pendingBuildCtx).(*oteltest.Span)
				Expect(span.Links()).To(ConsistOf(trace.Link{
					SpanContext: tracing.FromContext(ctx).SpanContext(),
				}))
				Expect(span.ParentSpanID()).To(Equal(tracing.FromContext(inputCtx1).SpanContext().SpanID()))
			})
		})

		Context("when the job inputs fail to fetch", func() {
			BeforeEach(func() {
				fakeJob.AlgorithmInputsReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(scheduleErr).To(Equal(fmt.Errorf("inputs: %w", disaster)))
			})
		})
	})
})
