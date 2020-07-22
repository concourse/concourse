package exec_test

import (
	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/build/buildfakes"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/vars"
	"github.com/concourse/concourse/vars/varsfakes"
)

const plainString = "  pv  \n\n"

const yamlString = `
k1: yv1
k2: yv2
`

const jsonString = `
{
  "k1": "jv1", "k2": "jv2"
}
`

var _ = Describe("LoadVarStep", func() {

	var (
		ctx        context.Context
		cancel     func()
		testLogger *lagertest.TestLogger

		fakeDelegate     *execfakes.FakeBuildStepDelegate
		fakeWorkerClient *workerfakes.FakeClient

		loadVarPlan        *atc.LoadVarPlan
		artifactRepository *build.Repository
		state              *execfakes.FakeRunState
		fakeSource         *buildfakes.FakeRegisterableArtifact

		spStep  exec.Step
		stepErr error

		credVarsTracker vars.CredVarsTracker

		stepMetadata = exec.StepMetadata{
			TeamID:       123,
			TeamName:     "some-team",
			BuildID:      42,
			BuildName:    "some-build",
			PipelineID:   4567,
			PipelineName: "some-pipeline",
		}

		stdout, stderr *gbytes.Buffer

		planID = 56
	)

	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("var-step-test")
		ctx, cancel = context.WithCancel(context.Background())
		ctx = lagerctx.NewContext(ctx, testLogger)

		credVars := vars.StaticVariables{}
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

		fakeWorkerClient = new(workerfakes.FakeClient)
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		plan := atc.Plan{
			ID:      atc.PlanID(planID),
			LoadVar: loadVarPlan,
		}

		spStep = exec.NewLoadVarStep(
			plan.ID,
			*plan.LoadVar,
			stepMetadata,
			fakeDelegate,
			fakeWorkerClient,
		)

		stepErr = spStep.Run(ctx, state)
	})

	Context("when format is specified", func() {
		Context("when format is invalid", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Format: "diff",
				}
			})

			It("step should fail", func() {
				Expect(stepErr).To(HaveOccurred())
				Expect(stepErr.Error()).To(Equal("invalid format diff"))
			})
		})

		Context("when format is trim", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Format: "trim",
				}

				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("step should not fail", func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})

			It("should var parsed correctly", func() {
				value, err := vars.NewTemplate([]byte("((.:some-var))")).Evaluate(credVarsTracker, vars.EvaluateOpts{})
				Expect(err).ToNot(HaveOccurred())
				Expect(string(value)).To(Equal("pv\n"))
			})
		})

		Context("when format is raw", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Format: "raw",
				}

				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("step should not fail", func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})

			It("should var parsed correctly", func() {
				value, err := vars.NewTemplate([]byte("((.:some-var))")).Evaluate(credVarsTracker, vars.EvaluateOpts{})
				Expect(err).ToNot(HaveOccurred())
				Expect(string(value)).To(Equal("\"  pv  \\n\\n\"\n"))
			})
		})

		Context("when format is json", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Format: "json",
				}

				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: jsonString}, nil)
			})

			It("step should not fail", func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})

			It("should var parsed correctly", func() {
				value, err := vars.NewTemplate([]byte("((.:some-var.k1))((.:some-var.k2))")).Evaluate(credVarsTracker, vars.EvaluateOpts{})
				Expect(err).ToNot(HaveOccurred())
				Expect(string(value)).To(Equal("jv1jv2\n"))
			})
		})

		Context("when format is yml", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Format: "yml",
				}

				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: yamlString}, nil)
			})

			It("step should not fail", func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})

			It("should var parsed correctly", func() {
				value, err := vars.NewTemplate([]byte("((.:some-var.k1))((.:some-var.k2))")).Evaluate(credVarsTracker, vars.EvaluateOpts{})
				Expect(err).ToNot(HaveOccurred())
				Expect(string(value)).To(Equal("yv1yv2\n"))
			})
		})

		Context("when format is yaml", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Format: "yaml",
				}

				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: yamlString}, nil)
			})

			It("step should not fail", func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})

			It("should var parsed correctly", func() {
				value, err := vars.NewTemplate([]byte("((.:some-var.k1))((.:some-var.k2))")).Evaluate(credVarsTracker, vars.EvaluateOpts{})
				Expect(err).ToNot(HaveOccurred())
				Expect(string(value)).To(Equal("yv1yv2\n"))
			})
		})
	})

	Context("when format is not specified", func() {
		Context("when file extension is other than json, yml and yaml", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.diff",
				}

				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("step should not fail", func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})

			It("should var parsed correctly as trim", func() {
				value, err := vars.NewTemplate([]byte("((.:some-var))")).Evaluate(credVarsTracker, vars.EvaluateOpts{})
				Expect(err).ToNot(HaveOccurred())
				Expect(string(value)).To(Equal("pv\n"))
			})
		})

		Context("when format is json", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.json",
				}

				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: jsonString}, nil)
			})

			It("step should not fail", func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})

			It("should var parsed correctly", func() {
				value, err := vars.NewTemplate([]byte("((.:some-var.k1))((.:some-var.k2))")).Evaluate(credVarsTracker, vars.EvaluateOpts{})
				Expect(err).ToNot(HaveOccurred())
				Expect(string(value)).To(Equal("jv1jv2\n"))
			})
		})

		Context("when format is yml", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.yml",
				}

				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: yamlString}, nil)
			})

			It("step should not fail", func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})

			It("should var parsed correctly", func() {
				value, err := vars.NewTemplate([]byte("((.:some-var.k1))((.:some-var.k2))")).Evaluate(credVarsTracker, vars.EvaluateOpts{})
				Expect(err).ToNot(HaveOccurred())
				Expect(string(value)).To(Equal("yv1yv2\n"))
			})
		})

		Context("when format is yaml", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.yaml",
				}

				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: yamlString}, nil)
			})

			It("step should not fail", func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})

			It("should var parsed correctly", func() {
				value, err := vars.NewTemplate([]byte("((.:some-var.k1))((.:some-var.k2))")).Evaluate(credVarsTracker, vars.EvaluateOpts{})
				Expect(err).ToNot(HaveOccurred())
				Expect(string(value)).To(Equal("yv1yv2\n"))
			})
		})
	})

	Context("when file is bad", func() {
		Context("when json file is bad", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.json",
				}

				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("step should fail", func() {
				Expect(stepErr).To(HaveOccurred())
				Expect(stepErr).To(MatchError(ContainSubstring("failed to parse some-resource/a.json in format json")))
			})
		})

		Context("when yaml file is bad", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.yaml",
				}

				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: "a:\nb"}, nil)
			})

			It("step should fail", func() {
				Expect(stepErr).To(HaveOccurred())
				Expect(stepErr).To(MatchError(ContainSubstring("failed to parse some-resource/a.yaml in format yaml")))
			})
		})
	})

	Context("reveal", func() {
		var fakeCredVarsTracker *varsfakes.FakeCredVarsTracker

		BeforeEach(func() {
			fakeCredVarsTracker = new(varsfakes.FakeCredVarsTracker)
			fakeDelegate.VariablesReturns(fakeCredVarsTracker)
		})

		Context("when reveal is not specified", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.diff",
				}
				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("local var should be redacted", func() {
				_, _, redact := fakeCredVarsTracker.AddLocalVarArgsForCall(0)
				Expect(redact).To(BeTrue())
			})
		})

		Context("when reveal is false", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Reveal: false,
				}
				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("local var should be redacted", func() {
				_, _, redact := fakeCredVarsTracker.AddLocalVarArgsForCall(0)
				Expect(redact).To(BeTrue())
			})
		})

		Context("when reveal is true", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Reveal: true,
				}
				fakeWorkerClient.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("local var should not be redacted", func() {
				_, _, redact := fakeCredVarsTracker.AddLocalVarArgsForCall(0)
				Expect(redact).To(BeFalse())
			})
		})
	})
})
