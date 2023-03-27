package exec_test

import (
	"context"
	"encoding/json"
	"strings"

	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	"github.com/concourse/concourse/tracing"
)

const plainString = "  pv  \n\n"

const yamlString = `
k1: yv1
k2: yv2
k3: 123
`

const jsonString = `
{
  "k1": "jv1", "k2": "jv2", "k3": 123
}
`

var _ = Describe("LoadVarStep", func() {

	var (
		ctx        context.Context
		cancel     func()
		testLogger *lagertest.TestLogger

		fakeDelegate        *execfakes.FakeBuildStepDelegate
		fakeDelegateFactory *execfakes.FakeBuildStepDelegateFactory

		fakeStreamer *execfakes.FakeStreamer

		spanCtx context.Context

		loadVarPlan        *atc.LoadVarPlan
		artifactRepository *build.Repository
		state              *execfakes.FakeRunState

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

		artifactRepository.RegisterArtifact("some-resource", runtimetest.NewVolume("some-handle"), false)

		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()

		fakeDelegate = new(execfakes.FakeBuildStepDelegate)
		fakeDelegate.StdoutReturns(stdout)
		fakeDelegate.StderrReturns(stderr)

		spanCtx = context.Background()
		fakeDelegate.StartSpanReturns(spanCtx, tracing.NoopSpan)

		fakeDelegateFactory = new(execfakes.FakeBuildStepDelegateFactory)
		fakeDelegateFactory.BuildStepDelegateReturns(fakeDelegate)

		fakeStreamer = new(execfakes.FakeStreamer)
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
			fakeStreamer,
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

				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("var should be parsed correctly", func() {
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

				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("var should be parsed correctly", func() {
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

				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: jsonString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("var should be parsed correctly", func() {
				expectLocalVarAdded("some-var", map[string]interface{}{"k1": "jv1", "k2": "jv2", "k3": json.Number("123")}, true)
			})
		})

		Context("when format is yml", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Format: "yml",
				}

				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: yamlString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("var should be parsed correctly", func() {
				expectLocalVarAdded("some-var", map[string]interface{}{"k1": "yv1", "k2": "yv2", "k3": json.Number("123")}, true)
			})
		})

		Context("when format is yaml", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name:   "some-var",
					File:   "some-resource/a.diff",
					Format: "yaml",
				}

				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: yamlString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("var should be parsed correctly", func() {
				expectLocalVarAdded("some-var", map[string]interface{}{"k1": "yv1", "k2": "yv2", "k3": json.Number("123")}, true)
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

				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("var should be parsed correctly as trim", func() {
				expectLocalVarAdded("some-var", strings.TrimSpace(plainString), true)
			})
		})

		Context("when format is json", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.json",
				}

				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: jsonString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("var should be parsed correctly", func() {
				expectLocalVarAdded("some-var", map[string]interface{}{"k1": "jv1", "k2": "jv2", "k3": json.Number("123")}, true)
			})
		})

		Context("when format is yml", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.yml",
				}

				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: yamlString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("var should be parsed correctly", func() {
				expectLocalVarAdded("some-var", map[string]interface{}{"k1": "yv1", "k2": "yv2", "k3": json.Number("123")}, true)
			})
		})

		Context("when format is yaml", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.yaml",
				}

				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: yamlString}, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("var should be parsed correctly", func() {
				expectLocalVarAdded("some-var", map[string]interface{}{"k1": "yv1", "k2": "yv2", "k3": json.Number("123")}, true)
			})
		})
	})

	Context("when file is bad", func() {
		Context("when json file is invalid JSON", func() {
			BeforeEach(func() {
				loadVarPlan = &atc.LoadVarPlan{
					Name: "some-var",
					File: "some-resource/a.json",
				}

				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: jsonString + "{}"}, nil)
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

				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: "a:\nb"}, nil)
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

				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("step should fail", func() {
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
				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: plainString}, nil)
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
				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: plainString}, nil)
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
				fakeStreamer.StreamFileReturns(&fakeReadCloser{str: plainString}, nil)
			})

			It("local var should not be redacted", func() {
				expectLocalVarAdded("some-var", strings.TrimSpace(plainString), false)
			})
		})
	})
})
