package engine

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/policy/policyfakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	"github.com/concourse/concourse/vars"
)

var noopStepper exec.Stepper = func(atc.Plan) exec.Step {
	Fail("cannot create substep")
	return nil
}

var _ = Describe("TaskDelegate", func() {
	var (
		logger            *lagertest.TestLogger
		fakeBuild         *dbfakes.FakeBuild
		fakeClock         *fakeclock.FakeClock
		fakePolicyChecker *policyfakes.FakeChecker
		fakeWorkerFactory *dbfakes.FakeWorkerFactory
		fakeLockFactory   *lockfakes.FakeLockFactory

		state exec.RunState

		now = time.Date(1991, 6, 3, 5, 30, 0, 0, time.UTC)

		delegate *taskDelegate
		planID   = atc.PlanID("some-plan-id")

		exitStatus exec.ExitStatus
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeBuild = new(dbfakes.FakeBuild)
		fakeClock = fakeclock.NewFakeClock(now)
		credVars := vars.StaticVariables{
			"source-param": "super-secret-source",
			"git-key":      "{\n123\n456\n789\n}\n",
		}
		state = exec.NewRunState(noopStepper, credVars, true)

		fakePolicyChecker = new(policyfakes.FakeChecker)
		fakeWorkerFactory = new(dbfakes.FakeWorkerFactory)
		fakeLockFactory = new(lockfakes.FakeLockFactory)

		delegate = NewTaskDelegate(fakeBuild, planID, state, fakeClock, fakePolicyChecker, fakeWorkerFactory, fakeLockFactory).(*taskDelegate)

		delegate.SetTaskConfig(atc.TaskConfig{
			Platform: "some-platform",
			Run: atc.TaskRunConfig{
				Path: "some-foo-path",
				Dir:  "some-bar-dir",
			},
		})
	})

	Describe("Initializing", func() {
		JustBeforeEach(func() {
			delegate.Initializing(logger)
		})

		It("saves an event", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			event := fakeBuild.SaveEventArgsForCall(0)
			Expect(event.EventType()).To(Equal(atc.EventType("initialize-task")))
		})

		It("calls SaveEvent with the taskConfig", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			event := fakeBuild.SaveEventArgsForCall(0)
			Expect(json.Marshal(event)).To(MatchJSON(`{
				"time": 675927000,
				"origin": {"id": "some-plan-id"},
				"config": {
					"platform": "some-platform",
					"image":"",
					"run": {
						"path": "some-foo-path",
						"args": null,
						"dir": "some-bar-dir"
					},
					"inputs":null
				}
			}`))
		})
	})

	Describe("Starting", func() {
		JustBeforeEach(func() {
			delegate.Starting(logger)
		})

		It("saves an event", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			event := fakeBuild.SaveEventArgsForCall(0)
			Expect(event.EventType()).To(Equal(atc.EventType("start-task")))
		})

		It("calls SaveEvent with the taskConfig", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			event := fakeBuild.SaveEventArgsForCall(0)
			Expect(json.Marshal(event)).To(MatchJSON(`{
				"time": 675927000,
				"origin": {"id": "some-plan-id"},
				"config": {
					"platform": "some-platform",
					"image":"",
					"run": {
						"path": "some-foo-path",
						"args": null,
						"dir": "some-bar-dir"
					},
					"inputs":null
				}
			}`))
		})
	})

	Describe("Finished", func() {
		JustBeforeEach(func() {
			delegate.Finished(logger, exitStatus)
		})

		It("saves an event", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			event := fakeBuild.SaveEventArgsForCall(0)
			Expect(event.EventType()).To(Equal(atc.EventType("finish-task")))
		})
	})

	Describe("FetchImage", func() {
		var delegate exec.TaskDelegate

		var expectedCheckPlan, expectedGetPlan atc.Plan
		var types atc.ResourceTypes
		var imageResource atc.ImageResource

		var volume *runtimetest.Volume
		var fakeResourceCache *dbfakes.FakeResourceCache

		var runPlans []atc.Plan
		var stepper exec.Stepper

		var tags []string
		var privileged bool

		var imageSpec runtime.ImageSpec
		var fetchErr error

		BeforeEach(func() {
			atc.DefaultCheckInterval = 1 * time.Minute
			volume = runtimetest.NewVolume("some-volume")

			runPlans = nil
			stepper = func(p atc.Plan) exec.Step {
				runPlans = append(runPlans, p)

				step := new(execfakes.FakeStep)
				fakeResourceCache = new(dbfakes.FakeResourceCache)
				step.RunStub = func(_ context.Context, state exec.RunState) (bool, error) {
					if p.Get != nil {
						state.ArtifactRepository().RegisterArtifact("image", volume, false)
						state.StoreResult(expectedGetPlan.ID, exec.GetResult{
							Name:          "image",
							ResourceCache: fakeResourceCache,
						})
					}
					return true, nil
				}
				return step
			}

			runState := exec.NewRunState(stepper, nil, false)
			delegate = NewTaskDelegate(fakeBuild, planID, runState, fakeClock, fakePolicyChecker, fakeWorkerFactory, fakeLockFactory)

			imageResource = atc.ImageResource{
				Type:   "docker",
				Source: atc.Source{"some": "((source-var))"},
				Params: atc.Params{"some": "((params-var))"},
				Tags:   atc.Tags{"some", "tags"},
			}

			types = atc.ResourceTypes{
				{
					Name:   "some-custom-type",
					Type:   "another-custom-type",
					Source: atc.Source{"some-custom": "((source-var))"},
					Params: atc.Params{"some-custom": "((params-var))"},
				},
				{
					Name:       "another-custom-type",
					Type:       "registry-image",
					Source:     atc.Source{"another-custom": "((source-var))"},
					Privileged: true,
				},
			}

			expectedCheckPlan = atc.Plan{
				ID: planID + "/image-check",
				Check: &atc.CheckPlan{
					Name:   "image",
					Type:   "docker",
					Source: atc.Source{"some": "((source-var))"},
					TypeImage: atc.TypeImage{
						BaseType: "docker",
					},
					Tags: atc.Tags{"some", "tags"},
					Interval: atc.CheckEvery{
						Interval: 1 * time.Minute,
					},
				},
			}

			expectedGetPlan = atc.Plan{
				ID: planID + "/image-get",
				Get: &atc.GetPlan{
					Name:   "image",
					Type:   "docker",
					Source: atc.Source{"some": "((source-var))"},
					TypeImage: atc.TypeImage{
						BaseType: "docker",
					},
					VersionFrom: &expectedCheckPlan.ID,
					Params:      atc.Params{"some": "((params-var))"},
					Tags:        atc.Tags{"some", "tags"},
				},
			}
		})

		AfterEach(func() {
			atc.DefaultCheckInterval = 0
		})

		JustBeforeEach(func() {
			imageSpec, fetchErr = delegate.FetchImage(context.TODO(), imageResource, types, privileged, tags, false)
		})

		It("succeeds", func() {
			Expect(fetchErr).ToNot(HaveOccurred())
		})

		It("returns an image spec containing the artifact", func() {
			Expect(imageSpec).To(Equal(runtime.ImageSpec{
				ImageArtifact: volume,
				Privileged:    false,
			}))
		})

		It("generates and runs a check and get plan", func() {
			Expect(runPlans).To(Equal([]atc.Plan{
				expectedCheckPlan,
				expectedGetPlan,
			}))
		})

		It("sends events for image check and get", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(2))
			e := fakeBuild.SaveEventArgsForCall(0)
			Expect(e).To(Equal(event.ImageCheck{
				Time: 675927000,
				Origin: event.Origin{
					ID: event.OriginID(planID),
				},
				PublicPlan: expectedCheckPlan.Public(),
			}))

			e = fakeBuild.SaveEventArgsForCall(1)
			Expect(e).To(Equal(event.ImageGet{
				Time: 675927000,
				Origin: event.Origin{
					ID: event.OriginID(planID),
				},
				PublicPlan: expectedGetPlan.Public(),
			}))
		})

		Context("when the check plan is nil", func() {
			BeforeEach(func() {
				imageResource.Version = atc.Version{"some": "version"}
				expectedGetPlan.Get.Version = &atc.Version{"some": "version"}
			})

			It("only saves an ImageGet event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				e := fakeBuild.SaveEventArgsForCall(0)
				Expect(e).To(Equal(event.ImageGet{
					Time: 675927000,
					Origin: event.Origin{
						ID: event.OriginID(planID),
					},
					PublicPlan: expectedGetPlan.Public(),
				}))
			})
		})
	})
})
