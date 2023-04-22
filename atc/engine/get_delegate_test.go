package engine_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/policy/policyfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("GetDelegate", func() {
	var (
		logger            *lagertest.TestLogger
		fakeBuild         *dbfakes.FakeBuild
		fakePipeline      *dbfakes.FakePipeline
		fakeResource      *dbfakes.FakeResource
		fakeClock         *fakeclock.FakeClock
		fakePolicyChecker *policyfakes.FakeChecker

		state exec.RunState

		now        = time.Date(1991, 6, 3, 5, 30, 0, 0, time.UTC)
		delegate   exec.GetDelegate
		info       resource.VersionResult
		exitStatus exec.ExitStatus
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeBuild = new(dbfakes.FakeBuild)
		fakePipeline = new(dbfakes.FakePipeline)
		fakeResource = new(dbfakes.FakeResource)
		fakeClock = fakeclock.NewFakeClock(now)
		credVars := vars.StaticVariables{
			"source-param": "super-secret-source",
			"git-key":      "{\n123\n456\n789\n}\n",
		}
		state = exec.NewRunState(noopStepper, credVars, true)

		info = resource.VersionResult{
			Version:  atc.Version{"foo": "bar"},
			Metadata: []atc.MetadataField{{Name: "baz", Value: "shmaz"}},
		}

		fakePolicyChecker = new(policyfakes.FakeChecker)

		delegate = engine.NewGetDelegate(fakeBuild, "some-plan-id", state, fakeClock, fakePolicyChecker)
	})

	Describe("Finished", func() {
		JustBeforeEach(func() {
			delegate.Finished(logger, exitStatus, info)
		})

		It("saves an event", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.FinishGet{
				Origin:          event.Origin{ID: event.OriginID("some-plan-id")},
				Time:            now.Unix(),
				ExitStatus:      int(exitStatus),
				FetchedVersion:  info.Version,
				FetchedMetadata: info.Metadata,
			}))
		})
	})

	Describe("UpdateResourceVersion", func() {
		var resourceName string

		JustBeforeEach(func() {
			delegate.UpdateResourceVersion(logger, resourceName, info)
		})

		BeforeEach(func() {
			resourceName = "some-resource"
		})

		Context("when retrieving the pipeline fails", func() {
			BeforeEach(func() {
				fakeBuild.PipelineReturns(nil, false, errors.New("nope"))
			})

			It("doesn't update the metadata", func() {
				Expect(fakeResource.UpdateMetadataCallCount()).To(Equal(0))
			})
		})

		Context("when retrieving the pipeline succeeds", func() {

			Context("when the pipeline is not found", func() {
				BeforeEach(func() {
					fakeBuild.PipelineReturns(nil, false, nil)
				})

				It("doesn't update the metadata", func() {
					Expect(fakeResource.UpdateMetadataCallCount()).To(Equal(0))
				})
			})

			Context("when the pipeline is found", func() {
				BeforeEach(func() {
					fakeBuild.PipelineReturns(fakePipeline, true, nil)
				})

				Context("when retrieving the resource fails", func() {
					BeforeEach(func() {
						fakePipeline.ResourceReturns(nil, false, errors.New("nope"))
					})

					It("doesn't update the metadata", func() {
						Expect(fakeResource.UpdateMetadataCallCount()).To(Equal(0))
					})
				})

				Context("when retrieving the resource succeeds", func() {

					It("retrives the resource by name", func() {
						Expect(fakePipeline.ResourceArgsForCall(0)).To(Equal("some-resource"))
					})

					Context("when the resource is not found", func() {
						BeforeEach(func() {
							fakePipeline.ResourceReturns(nil, false, nil)
						})

						It("doesn't update the metadata", func() {
							Expect(fakeResource.UpdateMetadataCallCount()).To(Equal(0))
						})
					})

					Context("when the resource is found", func() {
						BeforeEach(func() {
							fakePipeline.ResourceReturns(fakeResource, true, nil)
						})

						It("updates the metadata", func() {
							Expect(fakeResource.UpdateMetadataCallCount()).To(Equal(1))
							version, metadata := fakeResource.UpdateMetadataArgsForCall(0)
							Expect(version).To(Equal(info.Version))
							Expect(metadata).To(Equal(db.NewResourceConfigMetadataFields(info.Metadata)))
						})
					})
				})
			})
		})
	})
})
