package exec_test

import (
	"context"
	"strings"

	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/build/buildfakes"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/tracing"
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

		fakeDelegate        *execfakes.FakeBuildStepDelegate
		fakeDelegateFactory *execfakes.FakeBuildStepDelegateFactory

		fakeArtifactStreamer *workerfakes.FakeArtifactStreamer

		spanCtx context.Context

		loadVarPlan        *atc.LoadVarPlan
		artifactRepository *build.Repository
		state              *execfakes.FakeRunState
		fakeSource         *buildfakes.FakeRegisterableArtifact

		spStep  exec.Step
		stepOk  bool
		stepErr error

		stepMetadata = exec.StepMetadata{
			TeamID:       123,
			TeamName:     "some-team",
			BuildID:      42,
			BuildName:    "some-build",
			PipelineID:   4567,
			PipelineName: "some-pipeline",
		}

		stdout, stderr *gbytes.Buffer

		planID = "56"
	)

	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("var-step-test")
		ctx, cancel = context.WithCancel(context.Background())
		ctx = lagerctx.NewContext(ctx, testLogger)

		artifactRepository = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(artifactRepository)

		fakeSource = new(buildfakes.FakeRegisterableArtifact)
		artifactRepository.RegisterArtifact("some-resource", fakeSource)

		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()

		fakeDelegate = new(execfakes.FakeBuildStepDelegate)
		fakeDelegate.StdoutReturns(stdout)
		fakeDelegate.StderrReturns(stderr)

		spanCtx = context.Background()
		fakeDelegate.StartSpanReturns(spanCtx, tracing.NoopSpan)

		fakeDelegateFactory = new(execfakes.FakeBuildStepDelegateFactory)
		fakeDelegateFactory.BuildStepDelegateReturns(fakeDelegate)

		fakeArtifactStreamer = new(workerfakes.FakeArtifactStreamer)
	})

	expectLocalVarAdded := func(expectKey string, expectValue interface{}, expectRedact bool) {
		Expect(state.AddLocalVarCallCount()).To(Equal(1))
		k, v, redact := state.AddLocalVarArgsForCall(0)
		Expect(k).To(Equal(expectKey))
		Expect(v).To(Equal(expectValue))
		Expect(redact).To(Equal(expectRedact))
	}

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
			fakeDelegateFactory,
			fakeArtifactStreamer,
		)

		stepOk, stepErr = spStep.Run(ctx, state)
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

				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("should var parsed correctly", func() {
				expectLocalVarAdded("some-var", strings.TrimSpace(plainString), true)
			})
		})

		Context("when format is raw", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Format: "raw",
				}

				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("should var parsed correctly", func() {
				expectLocalVarAdded("some-var", plainString, true)
			})
		})

		Context("when format is json", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Format: "json",
				}

				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: jsonString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("should var parsed correctly", func() {
				expectLocalVarAdded("some-var", map[string]interface{}{"k1": "jv1", "k2": "jv2"}, true)
			})
		})

		Context("when format is yml", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Format: "yml",
				}

				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: yamlString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("should var parsed correctly", func() {
				expectLocalVarAdded("some-var", map[string]interface{}{"k1": "yv1", "k2": "yv2"}, true)
			})
		})

		Context("when format is yaml", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Format: "yaml",
				}

				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: yamlString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("should var parsed correctly", func() {
				expectLocalVarAdded("some-var", map[string]interface{}{"k1": "yv1", "k2": "yv2"}, true)
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

				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("should var parsed correctly as trim", func() {
				expectLocalVarAdded("some-var", strings.TrimSpace(plainString), true)
			})
		})

		Context("when format is json", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.json",
				}

				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: jsonString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("should var parsed correctly", func() {
				expectLocalVarAdded("some-var", map[string]interface{}{"k1": "jv1", "k2": "jv2"}, true)
			})
		})

		Context("when format is yml", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.yml",
				}

				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: yamlString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("should var parsed correctly", func() {
				expectLocalVarAdded("some-var", map[string]interface{}{"k1": "yv1", "k2": "yv2"}, true)
			})
		})

		Context("when format is yaml", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.yaml",
				}

				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: yamlString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("should var parsed correctly", func() {
				expectLocalVarAdded("some-var", map[string]interface{}{"k1": "yv1", "k2": "yv2"}, true)
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

				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
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

				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: "a:\nb"}, nil)
			})

			It("step should fail", func() {
				Expect(stepErr).To(HaveOccurred())
				Expect(stepErr).To(MatchError(ContainSubstring("failed to parse some-resource/a.yaml in format yaml")))
			})
		})

		Context("when file path artifact is not registered", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource-not-in-the-registry/a.json",
				}

				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("step should fail", func() {
				Expect(stepErr).To(HaveOccurred())
				Expect(stepErr).To(
					Equal(artifact.UnknownArtifactSourceError{
						Name: "some-resource-not-in-the-registry",
						Path: "a.json",
					}))
				Expect(stepErr).To(MatchError("unknown artifact source: 'some-resource-not-in-the-registry' in file path 'a.json'"))
			})
		})
	})

	Context("reveal", func() {
		Context("when reveal is not specified", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.diff",
				}
				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("local var should be redacted", func() {
				expectLocalVarAdded("some-var", strings.TrimSpace(plainString), true)
			})
		})

		Context("when reveal is false", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Reveal: false,
				}
				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("local var should be redacted", func() {
				expectLocalVarAdded("some-var", strings.TrimSpace(plainString), true)
			})
		})

		Context("when reveal is true", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Reveal: true,
				}
				fakeArtifactStreamer.StreamFileFromArtifactReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("local var should not be redacted", func() {
				expectLocalVarAdded("some-var", strings.TrimSpace(plainString), false)
			})
		})
	})
})
