package engine

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/policy/policyfakes"
	"github.com/concourse/concourse/atc/runtime/runtimefakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/vars"
)

var noopStepper exec.Stepper = func(atc.Plan) exec.Step {
	Fail("cannot create substep")
	return nil
}

var _ = Describe("TaskDelegate", func() {
	var (
		logger              *lagertest.TestLogger
		fakeBuild           *dbfakes.FakeBuild
		fakeClock           *fakeclock.FakeClock
		fakePolicyChecker   *policyfakes.FakeChecker
		fakeArtifactSourcer *workerfakes.FakeArtifactSourcer
		fakeWorkerFactory   *dbfakes.FakeWorkerFactory
		fakeLockFactory     *lockfakes.FakeLockFactory
		fakeSecrets         *credsfakes.FakeSecrets

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
		state = exec.NewRunState(noopStepper, true)

		fakePolicyChecker = new(policyfakes.FakeChecker)
		fakeArtifactSourcer = new(workerfakes.FakeArtifactSourcer)
		fakeWorkerFactory = new(dbfakes.FakeWorkerFactory)
		fakeLockFactory = new(lockfakes.FakeLockFactory)
		fakeSecrets = new(credsfakes.FakeSecrets)

		delegate = NewTaskDelegate(fakeBuild, planID, state, fakeClock, fakePolicyChecker, fakeArtifactSourcer, fakeWorkerFactory, fakeLockFactory, fakeSecrets).(*taskDelegate)

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
		var fakeClient *workerfakes.FakeClient
		var fakeStrategy *workerfakes.FakeContainerPlacementStrategy

		BeforeEach(func() {
			fakeClient = new(workerfakes.FakeClient)
			fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
		})

		JustBeforeEach(func() {
			delegate.Finished(logger, exitStatus, fakeStrategy, fakeClient)
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
		var varSources atc.VarSourceConfigs
		var imageResource atc.ImageResource

		var fakeArtifact *runtimefakes.FakeArtifact
		var fakeSource *workerfakes.FakeStreamableArtifactSource
		var fakeResourceCache *dbfakes.FakeResourceCache

		var runPlans []atc.Plan
		var stepper exec.Stepper

		var tags []string
		var privileged bool

		var imageSpec worker.ImageSpec
		var fetchErr error

		BeforeEach(func() {
			fakeArtifact = new(runtimefakes.FakeArtifact)

			runPlans = nil
			stepper = func(p atc.Plan) exec.Step {
				runPlans = append(runPlans, p)

				step := new(execfakes.FakeStep)
				fakeResourceCache = new(dbfakes.FakeResourceCache)
				step.RunStub = func(_ context.Context, state exec.RunState) (bool, error) {
					if p.Get != nil {
						state.ArtifactRepository().RegisterArtifact("image", fakeArtifact)
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
			delegate = NewTaskDelegate(fakeBuild, planID, runState, fakeClock, fakePolicyChecker, fakeArtifactSourcer, fakeWorkerFactory, fakeLockFactory, fakeSecrets)

			fakeSource = new(workerfakes.FakeStreamableArtifactSource)
			fakeArtifactSourcer.SourceImageReturns(fakeSource, nil)

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
					VarPlans: []atc.Plan{
						{
							ID: planID + "/image-check/source/var-1",
							GetVar: &atc.GetVarPlan{
								Path:   "source-var",
								Fields: []string{},
							},
						},
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
					VarPlans: []atc.Plan{
						{
							ID: planID + "/image-get/source/var-1",
							GetVar: &atc.GetVarPlan{
								Path:   "source-var",
								Fields: []string{},
							},
						},
						{
							ID: planID + "/image-get/params/var-1",
							GetVar: &atc.GetVarPlan{
								Path:   "params-var",
								Fields: []string{},
							},
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			imageSpec, fetchErr = delegate.FetchImage(context.TODO(), imageResource, types, varSources, privileged, tags)
		})

		It("succeeds", func() {
			Expect(fetchErr).ToNot(HaveOccurred())
		})

		It("returns an image spec containing the artifact", func() {
			Expect(imageSpec).To(Equal(worker.ImageSpec{
				ImageArtifactSource: fakeSource,
				Privileged:          false,
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

	Describe("FetchVariables/Get", func() {
		var (
			stepVariables      vars.Variables
			buildVariables     *build.Variables
			sources            atc.VarSourceConfigs
			varRef             vars.Reference
			getVarID           atc.PlanID
			expectedGetVarPlan atc.Plan

			childState *execfakes.FakeRunState

			fetchedVal string
			value      interface{}
			fetched    bool
			fetchErr   error
		)

		BeforeEach(func() {
			sources = atc.VarSourceConfigs{
				{
					Name: "some-var-source",
					Type: "registry-image",
					Config: map[string]interface{}{
						"var": "config",
					},
				},
				{
					Name: "other-var-source",
					Type: "registry-image",
					Config: map[string]interface{}{
						"var": "other-config",
					},
				},
			}

			getVarID = planID + "/get-var/some-var-source:path"

			expectedGetVarPlan = atc.Plan{
				ID: getVarID,
				GetVar: &atc.GetVarPlan{
					Name:   "some-var-source",
					Path:   "path",
					Type:   "registry-image",
					Source: atc.Source{"var": "config"},
				},
			}

			varRef = vars.Reference{
				Source: "some-var-source",
				Path:   "path",
			}
			fetchedVal = "fetched-value"

			state = new(execfakes.FakeRunState)
			childState = new(execfakes.FakeRunState)
			childState.VarSourceConfigsReturns(sources)
			childState.RunReturns(true, nil)
			childState.ResultStub = func(planID atc.PlanID, to interface{}) bool {
				Expect(planID).To(Equal(getVarID))

				if reflect.TypeOf(fetchedVal).AssignableTo(reflect.TypeOf(to).Elem()) {
					reflect.ValueOf(to).Elem().Set(reflect.ValueOf(fetchedVal))
					return true
				}

				return false
			}

			state.NewScopeReturns(childState)

			stepVariables = delegate.FetchVariables(context.TODO(), sources)
		})

		JustBeforeEach(func() {
			value, fetched, fetchErr = stepVariables.Get(varRef)
		})

		Context("when the var does not have a source (global vars)", func() {
			BeforeEach(func() {
				varRef.Source = ""

				fakeSecrets.NewSecretLookupPathsReturns(nil)
			})

			It("calls get off the global secrets", func() {
				Expect(fakeSecrets.GetCallCount()).To(Equal(1))
				Expect(fakeSecrets.GetArgsForCall(0)).To(Equal(varRef.Path))
			})
		})

		Context("when the var is found in the build vars", func() {
			BeforeEach(func() {
				varRef.Source = "."
				buildVariables.SetVar(".", "path", "fetched-value", true)
			})

			It("succeeds", func() {
				Expect(fetchErr).ToNot(HaveOccurred())
				Expect(fetched).To(BeTrue())
			})

			It("returns the value", func() {
				Expect(value).To(Equal("fetched-value"))
			})

			It("did not spawn get var sub step", func() {
				Expect(childState.RunCallCount()).To(Equal(0))
			})
		})

		Context("when the var uses a var source", func() {
			It("creates a new scope for the get var substep", func() {
				Expect(state.NewScopeCallCount()).To(Equal(1))
			})

			It("sets new var source configs for the child state", func() {
				Expect(childState.SetVarSourceConfigsCallCount()).To(Equal(1))
				Expect(childState.SetVarSourceConfigsArgsForCall(0)).To(Equal(sources))
			})

			It("saves a build event for the sub get var plan", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				e := fakeBuild.SaveEventArgsForCall(0)
				Expect(e).To(Equal(event.SubGetVar{
					Time: 675927000,
					Origin: event.Origin{
						ID: event.OriginID(planID),
					},
					PublicPlan: expectedGetVarPlan.Public(),
				}))
			})

			It("runs a GetVar plan to get the var value", func() {
				Expect(childState.RunCallCount()).To(Equal(1))

				_, plan := childState.RunArgsForCall(0)
				Expect(plan).To(Equal(expectedGetVarPlan))
			})

			It("succeeds", func() {
				Expect(fetchErr).ToNot(HaveOccurred())
				Expect(fetched).To(BeTrue())
			})

			It("returns the value", func() {
				Expect(value).To(Equal("fetched-value"))
			})

			Context("when the var source is not found", func() {
				BeforeEach(func() {
					sources = atc.VarSourceConfigs{
						{
							Name: "other-var-source",
							Type: "registry-image",
							Config: map[string]interface{}{
								"var": "other-config",
							},
						},
					}

					childState.VarSourceConfigsReturns(sources)
				})

				It("returns no matching var source error", func() {
					Expect(fetchErr).To(Equal(ErrNoMatchingVarSource{"some-var-source"}))
				})
			})
		})

		Context("when running the get var step fails", func() {
			BeforeEach(func() {
				childState.RunStub = func(ctx context.Context, plan atc.Plan) (bool, error) {
					return false, nil
				}
			})

			It("errors", func() {
				Expect(fetchErr).To(MatchError("get var failed"))
			})
		})

		Context("when no result is returned by the get var step", func() {
			BeforeEach(func() {
				childState.ResultReturns(false)
			})

			It("errors", func() {
				Expect(fetchErr).To(MatchError("get var did not return a value"))
			})
		})
	})
})

func containerSpecDummy() worker.ContainerSpec {
	cpu := uint64(1024)
	memory := uint64(1024)

	return worker.ContainerSpec{
		TeamID: 123,
		ImageSpec: worker.ImageSpec{
			ImageArtifactSource: new(workerfakes.FakeStreamableArtifactSource),
			Privileged:          false,
		},
		Limits: worker.ContainerLimits{
			CPU:    &cpu,
			Memory: &memory,
		},
		Dir:     "some-artifact-root",
		Env:     []string{"SECURE=secret-task-param"},
		Inputs:  []worker.InputSource{},
		Outputs: worker.OutputPaths{},
	}
}

func workerSpecDummy() worker.WorkerSpec {
	return worker.WorkerSpec{
		TeamID:   123,
		Platform: "some-platform",
		Tags:     []string{"step", "tags"},
	}
}

func containerOwnerDummy() db.ContainerOwner {
	return db.NewBuildStepContainerOwner(
		1234,
		atc.PlanID("42"),
		123,
	)
}

func workerStub() *dbfakes.FakeWorker {
	fakeWorker := new(dbfakes.FakeWorker)
	fakeWorker.NameReturns("some-worker")

	activeTasks := 0
	fakeWorker.IncreaseActiveTasksStub = func() (int, error) {
		activeTasks++
		return activeTasks, nil
	}
	fakeWorker.DecreaseActiveTasksStub = func() (int, error) {
		activeTasks--
		return activeTasks, nil
	}
	fakeWorker.ActiveTasksStub = func() (int, error) {
		return activeTasks, nil
	}
	return fakeWorker
}
