package exec_test

import (
	"context"
	"errors"
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/policy/policyfakes"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("SetPipelineStep", func() {

	const badPipelineContentWithInvalidSyntax = `
---
jobs:
- name:
`

	const badPipelineContentWithEmptyContent = `
---
`

	const pipelineContent = `
---
jobs:
- name: some-job
  plan:
  - task: some-task
    config:
      platform: linux
      image_resource:
        type: registry-image
        source: {repository: busybox}
      run:
        path: echo
        args:
         - hello
`

	var pipelineObject = atc.Config{
		Jobs: atc.JobConfigs{
			{
				Name: "some-job",
				PlanSequence: []atc.Step{
					{
						Config: &atc.TaskStep{
							Name: "some-task",
							Config: &atc.TaskConfig{
								Platform: "linux",
								ImageResource: &atc.ImageResource{
									Type:   "registry-image",
									Source: atc.Source{"repository": "busybox"},
								},
								Run: atc.TaskRunConfig{
									Path: "echo",
									Args: []string{"hello"},
								},
							},
						},
					},
				},
			},
		},
	}

	var (
		ctx        context.Context
		cancel     func()
		testLogger *lagertest.TestLogger

		fakeTeamFactory  *dbfakes.FakeTeamFactory
		fakeBuildFactory *dbfakes.FakeBuildFactory
		fakeBuild        *dbfakes.FakeBuild
		fakeTeam         *dbfakes.FakeTeam
		fakePipeline     *dbfakes.FakePipeline
		spanCtx          context.Context

		fakeDelegate        *execfakes.FakeSetPipelineStepDelegate
		fakeDelegateFactory *execfakes.FakeSetPipelineStepDelegateFactory

		fakeAgent *policyfakes.FakeAgent

		fakeStreamer *execfakes.FakeStreamer

		spPlan             *atc.SetPipelinePlan
		artifactRepository *build.Repository
		state              *execfakes.FakeRunState

		spStep  exec.Step
		stepOk  bool
		stepErr error

		stepMetadata = exec.StepMetadata{
			TeamID:               123,
			TeamName:             "some-team",
			JobID:                87,
			JobName:              "some-job",
			BuildID:              42,
			BuildName:            "some-build",
			PipelineID:           4567,
			PipelineName:         "some-pipeline",
			PipelineInstanceVars: atc.InstanceVars{"branch": "feature/foo"},
		}

		stdout, stderr *gbytes.Buffer

		planID = "56"
	)

	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("set-pipeline-action-test")
		ctx, cancel = context.WithCancel(context.Background())
		ctx = lagerctx.NewContext(ctx, testLogger)

		artifactRepository = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(artifactRepository)

		state.GetStub = vars.StaticVariables{"source-param": "super-secret-source"}.Get

		artifactRepository.RegisterArtifact("some-resource", runtimetest.NewVolume("some-handle"), false)

		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()

		fakeDelegate = new(execfakes.FakeSetPipelineStepDelegate)
		fakeDelegate.StdoutReturns(stdout)
		fakeDelegate.StderrReturns(stderr)

		spanCtx = context.Background()
		fakeDelegate.StartSpanReturns(spanCtx, tracing.NoopSpan)

		fakeDelegateFactory = new(execfakes.FakeSetPipelineStepDelegateFactory)
		fakeDelegateFactory.SetPipelineStepDelegateReturns(fakeDelegate)

		fakeTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeBuildFactory = new(dbfakes.FakeBuildFactory)
		fakeBuild = new(dbfakes.FakeBuild)
		fakeTeam = new(dbfakes.FakeTeam)
		fakePipeline = new(dbfakes.FakePipeline)

		stepMetadata = exec.StepMetadata{
			TeamID:               123,
			TeamName:             "some-team",
			BuildID:              42,
			BuildName:            "some-build",
			PipelineID:           4567,
			PipelineName:         "some-pipeline",
			PipelineInstanceVars: atc.InstanceVars{"branch": "feature/foo"},
		}

		fakeTeam.IDReturns(stepMetadata.TeamID)
		fakeTeam.NameReturns(stepMetadata.TeamName)

		fakePipeline.NameReturns("some-pipeline")
		fakePipeline.InstanceVarsReturns(atc.InstanceVars{"branch": "feature/foo"})
		fakeTeamFactory.GetByIDReturns(fakeTeam)
		fakeBuildFactory.BuildReturns(fakeBuild, true, nil)

		fakeAgent = new(policyfakes.FakeAgent)
		fakeAgent.CheckReturns(policy.PassedPolicyCheck(), nil)
		fakePolicyAgentFactory.NewAgentReturns(fakeAgent, nil)

		fakeStreamer = new(execfakes.FakeStreamer)

		spPlan = &atc.SetPipelinePlan{
			Name:         "some-pipeline",
			File:         "some-resource/pipeline.yml",
			InstanceVars: atc.InstanceVars{"branch": "feature/foo"},
		}
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		plan := atc.Plan{
			ID:          atc.PlanID(planID),
			SetPipeline: spPlan,
		}

		spStep = exec.NewSetPipelineStep(
			plan.ID,
			*plan.SetPipeline,
			stepMetadata,
			fakeDelegateFactory,
			fakeTeamFactory,
			fakeBuildFactory,
			fakeStreamer,
		)

		stepOk, stepErr = spStep.Run(ctx, state)
	})

	Context("when file is not configured", func() {
		BeforeEach(func() {
			spPlan = &atc.SetPipelinePlan{
				Name: "some-pipeline",
			}
		})

		It("should fail with error of file not configured", func() {
			Expect(stepErr).To(HaveOccurred())
			Expect(stepErr.Error()).To(Equal("file is not specified"))
		})
	})

	Context("when file is configured", func() {
		Context("pipeline file not exist", func() {
			BeforeEach(func() {
				fakeStreamer.StreamFileReturns(nil, errors.New("file not found"))
			})

			It("should fail with error of file not configured", func() {
				Expect(stepErr).To(MatchError("file not found"))
			})
		})

		Context("when pipeline file exists but bad syntax", func() {
			BeforeEach(func() {
				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: badPipelineContentWithInvalidSyntax}, nil)
			})

			It("should not return error", func() {
				Expect(stepErr).NotTo(HaveOccurred())
			})

			It("should stderr have error message", func() {
				Expect(stderr).To(gbytes.Say("invalid pipeline:"))
				Expect(stderr).To(gbytes.Say("- invalid jobs:"))
			})

			It("should finish unsuccessfully", func() {
				Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
				_, succeeded := fakeDelegate.FinishedArgsForCall(0)
				Expect(succeeded).To(BeFalse())
			})
		})

		Context("when pipeline file exists but is empty", func() {
			BeforeEach(func() {
				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: badPipelineContentWithEmptyContent}, nil)
			})

			It("should return an error", func() {
				Expect(stepErr).NotTo(HaveOccurred())
			})

			It("should log an error message", func() {
				Expect(stderr).To(gbytes.Say("pipeline must contain at least one job"))
			})

			It("should not update the job and build id", func() {
				Expect(fakePipeline.SetParentIDsCallCount()).To(Equal(0))
			})
		})

		Context("when pipeline file is good", func() {
			BeforeEach(func() {
				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: pipelineContent}, nil)
			})

			Context("when get pipeline fails", func() {
				BeforeEach(func() {
					fakeTeam.PipelineReturns(nil, false, errors.New("fail to get pipeline"))
				})

				It("should return error", func() {
					Expect(stepErr).To(HaveOccurred())
					Expect(stepErr.Error()).To(Equal("fail to get pipeline"))
				})
			})

			Context("when specified pipeline not found", func() {
				BeforeEach(func() {
					fakeTeam.PipelineReturns(nil, false, nil)
					fakeBuild.SavePipelineReturns(fakePipeline, true, nil)
				})

				It("should save the pipeline", func() {
					Expect(fakeBuild.SavePipelineCallCount()).To(Equal(1))
					ref, _, _, _, paused := fakeBuild.SavePipelineArgsForCall(0)
					Expect(ref).To(Equal(atc.PipelineRef{
						Name:         "some-pipeline",
						InstanceVars: atc.InstanceVars{"branch": "feature/foo"},
					}))
					Expect(paused).To(BeFalse())
				})

				It("should stdout have message", func() {
					Expect(stdout).To(gbytes.Say("done"))
				})
			})

			Context("when specified pipeline exists already", func() {
				BeforeEach(func() {
					fakeTeam.PipelineReturns(fakePipeline, true, nil)
					fakeBuild.SavePipelineReturns(fakePipeline, false, nil)
				})

				Context("when no diff", func() {
					BeforeEach(func() {
						fakePipeline.ConfigReturns(pipelineObject, nil)
						fakePipeline.SetParentIDsReturns(nil)
					})

					It("should log 'no changes to apply'", func() {
						Expect(stdout).To(gbytes.Say("no changes to apply."))
					})

					It("should send a set pipeline changed event", func() {
						Expect(fakeDelegate.SetPipelineChangedCallCount()).To(Equal(1))
						_, changed := fakeDelegate.SetPipelineChangedArgsForCall(0)
						Expect(changed).To(BeFalse())
					})

					It("should update the job and build id", func() {
						Expect(fakePipeline.SetParentIDsCallCount()).To(Equal(1))
						jobID, buildID := fakePipeline.SetParentIDsArgsForCall(0)
						Expect(jobID).To(Equal(stepMetadata.JobID))
						Expect(buildID).To(Equal(stepMetadata.BuildID))
					})
				})

				Context("when there are some diff", func() {
					BeforeEach(func() {
						pipelineObject.Jobs[0].PlanSequence[0].Config.(*atc.TaskStep).Config.Run.Args = []string{"hello world"}
						fakePipeline.ConfigReturns(pipelineObject, nil)
					})

					It("should log diff", func() {
						Expect(stdout).To(gbytes.Say("job some-job has changed:"))
					})

					It("should send a set pipeline changed event", func() {
						Expect(fakeDelegate.SetPipelineChangedCallCount()).To(Equal(1))
						_, changed := fakeDelegate.SetPipelineChangedArgsForCall(0)
						Expect(changed).To(BeTrue())
					})
				})

				Context("when policy check fails", func() {
					BeforeEach(func() {
						fakeDelegate.CheckRunSetPipelinePolicyReturns(errors.New("policy-check-error"))
					})

					It("should return error", func() {
						Expect(stepErr).To(HaveOccurred())
						Expect(stepErr.Error()).To(Equal("policy-check-error"))
					})
				})

				Context("when SavePipeline fails", func() {
					BeforeEach(func() {
						fakeBuild.SavePipelineReturns(nil, false, errors.New("failed to save"))
					})

					It("should return error", func() {
						Expect(stepErr).To(HaveOccurred())
						Expect(stepErr.Error()).To(Equal("failed to save"))
					})

					Context("due to the pipeline being set by a newer build", func() {
						BeforeEach(func() {
							fakeBuild.SavePipelineReturns(nil, false, db.ErrSetByNewerBuild)
						})
						It("logs a warning", func() {
							Expect(stderr).To(gbytes.Say("WARNING: the pipeline was not saved because it was already saved by a newer build"))
						})
						It("does not fail the step", func() {
							Expect(stepErr).ToNot(HaveOccurred())
							Expect(stepOk).To(BeTrue())
						})
					})
				})

				It("should save the pipeline un-paused", func() {
					Expect(fakeBuild.SavePipelineCallCount()).To(Equal(1))
					ref, _, _, _, paused := fakeBuild.SavePipelineArgsForCall(0)
					Expect(ref).To(Equal(atc.PipelineRef{
						Name:         "some-pipeline",
						InstanceVars: atc.InstanceVars{"branch": "feature/foo"},
					}))
					Expect(paused).To(BeFalse())
				})

				It("should stdout have message", func() {
					Expect(stdout).To(gbytes.Say("setting pipeline: some-pipeline"))
					Expect(stdout).To(gbytes.Say("done"))
				})

				It("should finish successfully", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
					_, succeeded := fakeDelegate.FinishedArgsForCall(0)
					Expect(succeeded).To(BeTrue())
				})
			})

			Context("when set-pipeline self", func() {
				BeforeEach(func() {
					spPlan = &atc.SetPipelinePlan{
						Name:         "self",
						File:         "some-resource/pipeline.yml",
						Team:         "foo-team",
						InstanceVars: atc.InstanceVars{"branch": "feature/foo"},
					}
					fakeBuild.SavePipelineReturns(fakePipeline, false, nil)
				})

				It("should save the pipeline itself", func() {
					Expect(fakeBuild.SavePipelineCallCount()).To(Equal(1))
					pipelineRef, _, _, _, _ := fakeBuild.SavePipelineArgsForCall(0)
					Expect(pipelineRef).To(Equal(atc.PipelineRef{
						Name:         "some-pipeline",
						InstanceVars: atc.InstanceVars{"branch": "feature/foo"},
					}))
				})

				It("should save to the current team", func() {
					Expect(fakeBuild.SavePipelineCallCount()).To(Equal(1))
					_, teamId, _, _, _ := fakeBuild.SavePipelineArgsForCall(0)
					Expect(teamId).To(Equal(fakeTeam.ID()))
				})

				It("should print an experimental message", func() {
					Expect(stderr).To(gbytes.Say("WARNING: 'set_pipeline: self' is experimental"))
					Expect(stderr).To(gbytes.Say("contribute to discussion #5732"))
					Expect(stderr).To(gbytes.Say("discussions/5732"))
				})
			})

			Context("when team is configured", func() {
				var (
					fakeUserCurrentTeam *dbfakes.FakeTeam
				)

				BeforeEach(func() {
					fakeUserCurrentTeam = new(dbfakes.FakeTeam)
					fakeUserCurrentTeam.IDReturns(111)
					fakeUserCurrentTeam.NameReturns("main")
					fakeUserCurrentTeam.AdminReturns(false)

					stepMetadata.TeamID = fakeUserCurrentTeam.ID()
					stepMetadata.TeamName = fakeUserCurrentTeam.Name()
					fakeTeamFactory.FindTeamReturnsOnCall(
						0,
						fakeUserCurrentTeam, true, nil,
					)
				})

				Context("when team is set to the empty string", func() {
					BeforeEach(func() {
						fakeBuild.PipelineReturns(fakePipeline, true, nil)
						fakeBuild.SavePipelineReturns(fakePipeline, false, nil)
						spPlan.Team = ""
					})

					It("should finish successfully", func() {
						Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
						_, succeeded := fakeDelegate.FinishedArgsForCall(0)
						Expect(succeeded).To(BeTrue())
					})
				})

				Context("when team does not exist", func() {
					BeforeEach(func() {
						spPlan.Team = "not-found"
						fakeTeamFactory.FindTeamReturnsOnCall(
							1,
							nil, false, nil,
						)
					})

					It("should return error", func() {
						Expect(stepErr).To(HaveOccurred())
						Expect(stepErr.Error()).To(Equal("team not-found not found"))
					})
				})

				Context("when team exists", func() {
					Context("when the target team is the current team", func() {
						BeforeEach(func() {
							spPlan.Team = fakeUserCurrentTeam.Name()
							fakeTeamFactory.FindTeamReturnsOnCall(
								1,
								fakeUserCurrentTeam, true, nil,
							)

							fakeBuild.PipelineReturns(fakePipeline, true, nil)
							fakeBuild.SavePipelineReturns(fakePipeline, false, nil)
						})

						It("should finish successfully", func() {
							_, teamID, _, _, _ := fakeBuild.SavePipelineArgsForCall(0)
							Expect(teamID).To(Equal(fakeUserCurrentTeam.ID()))
							Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
							_, succeeded := fakeDelegate.FinishedArgsForCall(0)
							Expect(succeeded).To(BeTrue())
						})

						It("should print an experimental message", func() {
							Expect(stderr).To(gbytes.Say("WARNING: specifying the team"))
							Expect(stderr).To(gbytes.Say("contribute to discussion #5731"))
							Expect(stderr).To(gbytes.Say("discussions/5731"))
						})
					})

					Context("when the team is not the current team", func() {
						BeforeEach(func() {
							spPlan.Team = fakeTeam.Name()
							fakeTeamFactory.FindTeamReturnsOnCall(
								1,
								fakeTeam, true, nil,
							)
						})

						Context("when the current team is an admin team", func() {
							BeforeEach(func() {
								fakeUserCurrentTeam.AdminReturns(true)

								fakeBuild.PipelineReturns(fakePipeline, true, nil)
								fakeBuild.SavePipelineReturns(fakePipeline, false, nil)
							})

							It("should finish successfully", func() {
								_, teamID, _, _, _ := fakeBuild.SavePipelineArgsForCall(0)
								Expect(teamID).To(Equal(fakeTeam.ID()))
								Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
								_, succeeded := fakeDelegate.FinishedArgsForCall(0)
								Expect(succeeded).To(BeTrue())
							})
						})

						Context("when the current team is not an admin team", func() {
							It("should return error", func() {

								Expect(stepErr).To(HaveOccurred())
								Expect(stepErr.Error()).To(Equal(
									"only main team can set another team's pipeline",
								))
							})
						})
					})
				})
			})
		})
	})
})

type fakeReadCloser struct {
	str   string
	index int
}

func (r *fakeReadCloser) Read(p []byte) (int, error) {
	if r.index >= len(r.str) {
		return 0, io.EOF
	}
	l := copy(p, []byte(r.str)[r.index:])
	r.index += l
	return l, nil
}

func (r *fakeReadCloser) Close() error {
	return nil
}
