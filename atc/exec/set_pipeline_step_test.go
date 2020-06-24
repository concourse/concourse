package exec_test

import (
	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	"context"
	"errors"
	"io"

	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/build/buildfakes"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("SetPipelineStep", func() {

	const badPipelineContentWithInvalidSyntax = `
---
jobs:
- name:
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

		fakeDelegate    *execfakes.FakeBuildStepDelegate
		fakeTeamFactory *dbfakes.FakeTeamFactory
		fakeTeam        *dbfakes.FakeTeam
		fakePipeline    *dbfakes.FakePipeline

		fakeWorkerClient *workerfakes.FakeClient

		spPlan             *atc.SetPipelinePlan
		artifactRepository *build.Repository
		state              *execfakes.FakeRunState
		fakeSource         *buildfakes.FakeRegisterableArtifact

		spStep  exec.Step
		stepErr error

		credVarsTracker vars.CredVarsTracker

		stepMetadata exec.StepMetadata

		stdout, stderr *gbytes.Buffer

		planID = 56
	)

	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("set-pipeline-action-test")
		ctx, cancel = context.WithCancel(context.Background())
		ctx = lagerctx.NewContext(ctx, testLogger)

		credVars := vars.StaticVariables{"source-param": "super-secret-source"}
		credVarsTracker = vars.NewCredVarsTracker(credVars, true)

		artifactRepository = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(artifactRepository)

		fakeSource = new(buildfakes.FakeRegisterableArtifact)
		artifactRepository.RegisterArtifact("some-resource", fakeSource)

		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()

		fakeDelegate = new(execfakes.FakeBuildStepDelegate)
		fakeDelegate.VariablesReturns(credVarsTracker)
		fakeDelegate.StdoutReturns(stdout)
		fakeDelegate.StderrReturns(stderr)

		fakeTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeTeam = new(dbfakes.FakeTeam)
		fakePipeline = new(dbfakes.FakePipeline)

		stepMetadata = exec.StepMetadata{
			TeamID:       123,
			TeamName:     "some-team",
			BuildID:      42,
			BuildName:    "some-build",
			PipelineID:   4567,
			PipelineName: "some-pipeline",
		}

		fakeTeam.IDReturns(stepMetadata.TeamID)
		fakeTeam.NameReturns(stepMetadata.TeamName)

		fakePipeline.NameReturns("some-pipeline")
		fakeTeamFactory.GetByIDReturns(fakeTeam)

		fakeWorkerClient = new(workerfakes.FakeClient)

		spPlan = &atc.SetPipelinePlan{
			Name: "some-pipeline",
			File: "some-resource/pipeline.yml",
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
			fakeDelegate,
			fakeTeamFactory,
			fakeWorkerClient,
		)

		stepErr = spStep.Run(ctx, state)
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
				fakeWorkerClient.StreamFileFromArtifactReturns(nil, errors.New("file not found"))
			})

			It("should fail with error of file not configured", func() {
				Expect(stepErr).To(HaveOccurred())
				Expect(stepErr.Error()).To(Equal("file not found"))
			})
		})

		Context("when pipeline file exists but bad syntax", func() {
			BeforeEach(func() {
				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: badPipelineContentWithInvalidSyntax}, nil)
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

		Context("when pipeline file is good", func() {
			BeforeEach(func() {
				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: pipelineContent}, nil)
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
					fakeTeam.SavePipelineReturns(fakePipeline, true, nil)
				})

				It("should save the pipeline un-paused", func() {
					Expect(fakeTeam.SavePipelineCallCount()).To(Equal(1))
					name, _, _, paused := fakeTeam.SavePipelineArgsForCall(0)
					Expect(name).To(Equal("some-pipeline"))
					Expect(paused).To(BeFalse())
				})

				It("should stdout have message", func() {
					Expect(stdout).To(gbytes.Say("done"))
				})
			})

			Context("when specified pipeline exists already", func() {
				BeforeEach(func() {
					fakeTeam.PipelineReturns(fakePipeline, true, nil)
					fakeTeam.SavePipelineReturns(fakePipeline, false, nil)
				})

				Context("when no diff", func() {
					BeforeEach(func() {
						fakePipeline.ConfigReturns(pipelineObject, nil)
					})

					It("should log no-diff", func() {
						Expect(stdout).To(gbytes.Say("no diff found."))
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
				})

				Context("when SavePipeline fails", func() {
					BeforeEach(func() {
						fakeTeam.SavePipelineReturns(nil, false, errors.New("failed to save"))
					})

					It("should return error", func() {
						Expect(stepErr).To(HaveOccurred())
						Expect(stepErr.Error()).To(Equal("failed to save"))
					})
				})

				It("should save the pipeline un-paused", func() {
					Expect(fakeTeam.SavePipelineCallCount()).To(Equal(1))
					name, _, _, paused := fakeTeam.SavePipelineArgsForCall(0)
					Expect(name).To(Equal("some-pipeline"))
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
						fakeTeam.PipelineReturns(fakePipeline, true, nil)
						fakeTeam.SavePipelineReturns(fakePipeline, false, nil)
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

							fakeUserCurrentTeam.PipelineReturns(fakePipeline, true, nil)
							fakeUserCurrentTeam.SavePipelineReturns(fakePipeline, false, nil)
						})

						It("should finish successfully", func() {
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

								fakeTeam.PipelineReturns(fakePipeline, true, nil)
								fakeTeam.SavePipelineReturns(fakePipeline, false, nil)
							})

							It("should finish successfully", func() {
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
