package builder_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/builder"
	"github.com/concourse/atc/builder/fakes"
	"github.com/concourse/atc/db"
	enginefakes "github.com/concourse/atc/engine/fakes"
)

var _ = Describe("Builder", func() {
	var (
		builderDB  *fakes.FakeBuilderDB
		fakeEngine *enginefakes.FakeEngine

		builder Builder

		build db.Build
		plan  atc.BuildPlan

		buildErr error
	)

	BeforeEach(func() {
		builderDB = new(fakes.FakeBuilderDB)

		fakeEngine = new(enginefakes.FakeEngine)
		fakeEngine.NameReturns("fake-engine")

		build = db.Build{
			ID:   128,
			Name: "some-build",
		}

		plan = atc.BuildPlan{
			Config: atc.BuildConfig{
				Image: "some-image",

				Params: map[string]string{
					"FOO": "1",
					"BAR": "2",
				},

				Run: atc.BuildRunConfig{
					Path: "some-script",
					Args: []string{"arg1", "arg2"},
				},
			},
		}

		builderDB.StartBuildReturns(true, nil)

		builder = NewBuilder(builderDB, fakeEngine)
	})

	JustBeforeEach(func() {
		buildErr = builder.Build(build, plan)
	})

	Context("when creating the build succeeds", func() {
		var fakeBuild *enginefakes.FakeBuild

		BeforeEach(func() {
			fakeBuild = new(enginefakes.FakeBuild)
			fakeBuild.MetadataReturns("some-metadata")

			fakeEngine.CreateBuildReturns(fakeBuild, nil)
		})

		It("succeeds", func() {
			Ω(buildErr).ShouldNot(HaveOccurred())
		})

		It("starts the build in the database", func() {
			Ω(builderDB.StartBuildCallCount()).Should(Equal(1))

			buildID, engine, metadata := builderDB.StartBuildArgsForCall(0)
			Ω(buildID).Should(Equal(128))
			Ω(engine).Should(Equal("fake-engine"))
			Ω(metadata).Should(Equal("some-metadata"))
		})

		Context("when the build fails to transition to started", func() {
			BeforeEach(func() {
				builderDB.StartBuildReturns(false, nil)
			})

			It("aborts the build", func() {
				Ω(fakeBuild.AbortCallCount()).Should(Equal(1))
			})
		})
	})

	Context("when creating the build fails", func() {
		disaster := errors.New("failed")

		BeforeEach(func() {
			fakeEngine.CreateBuildReturns(nil, disaster)
		})

		It("returns the error", func() {
			Ω(buildErr).Should(Equal(disaster))
		})

		It("does not start the build", func() {
			Ω(builderDB.StartBuildCallCount()).Should(Equal(0))
		})
	})
})
