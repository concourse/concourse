package db_test

import (
	"errors"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/db"
	"github.com/concourse/atc/db/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PlanConvertingConfigDB", func() {
	var nestedDB *fakes.FakeConfigDB
	var configDB PlanConvertingConfigDB

	pipelineName := "pipeline-name"

	planBasedConfig := atc.Config{
		Jobs: atc.JobConfigs{
			{
				Name: "some-job",
				Plan: atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{Get: "some-input"},
						},
					},
					{
						Task:           "build",
						TaskConfigPath: "some/config/path.yml",
						TaskConfig: &atc.TaskConfig{
							Run: atc.TaskRunConfig{
								Path: "ls",
							},
						},
					},
					{
						Aggregate: &atc.PlanSequence{
							{Put: "some-output"},
						},
					},
				},
			},
		},
	}

	buildBasedConfig := atc.Config{
		Jobs: atc.JobConfigs{
			{
				Name: "some-job",
				InputConfigs: []atc.JobInputConfig{
					{Resource: "some-input"},
				},
				TaskConfigPath: "some/config/path.yml",
				TaskConfig: &atc.TaskConfig{
					Run: atc.TaskRunConfig{
						Path: "ls",
					},
				},
				OutputConfigs: []atc.JobOutputConfig{
					{Resource: "some-output"},
				},
			},
		},
	}

	BeforeEach(func() {
		nestedDB = new(fakes.FakeConfigDB)
		configDB = PlanConvertingConfigDB{nestedDB}
	})

	Describe("GetConfig", func() {
		var gotConfig atc.Config
		var gotVersion ConfigVersion
		var getErr error

		JustBeforeEach(func() {
			gotConfig, gotVersion, getErr = configDB.GetConfig(pipelineName)
		})

		It("calls GetConfig with the correct arguments", func() {
			Ω(nestedDB.GetConfigCallCount()).Should(Equal(1))

			name := nestedDB.GetConfigArgsForCall(0)
			Ω(name).Should(Equal(pipelineName))
		})

		Context("when the nested config db yields a config containing jobs with plans", func() {
			BeforeEach(func() {
				nestedDB.GetConfigReturns(planBasedConfig, 42, nil)
			})

			It("succeeds", func() {
				Ω(getErr).ShouldNot(HaveOccurred())
			})

			It("returns the config ID", func() {
				Ω(gotVersion).Should(Equal(ConfigVersion(42)))
			})

			It("returns the config as-is", func() {
				Ω(gotConfig).Should(Equal(planBasedConfig))
			})
		})

		Context("when the nested config db yields a config containing jobs with inputs/outputs/build", func() {
			BeforeEach(func() {
				nestedDB.GetConfigReturns(buildBasedConfig, 42, nil)
			})

			It("succeeds", func() {
				Ω(getErr).ShouldNot(HaveOccurred())
			})

			It("returns the config ID", func() {
				Ω(gotVersion).Should(Equal(ConfigVersion(42)))
			})

			It("returns the config with the job converted to using plans", func() {
				Ω(gotConfig).Should(Equal(planBasedConfig))
			})
		})

		Context("when the nested config db fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				nestedDB.GetConfigReturns(atc.Config{}, 0, disaster)
			})

			It("returns the error", func() {
				Ω(getErr).Should(Equal(disaster))
			})
		})
	})

	Context("SaveConfig", func() {
		var configToSave atc.Config
		var versionToSave ConfigVersion
		var pausedState PipelinePausedState

		var saveErr error

		BeforeEach(func() {
			configToSave = atc.Config{}
			versionToSave = 42
			pausedState = PipelinePaused
		})

		JustBeforeEach(func() {
			saveErr = configDB.SaveConfig(pipelineName, configToSave, versionToSave, pausedState)
		})

		Context("when the given config contains jobs with inputs/outputs/build", func() {
			BeforeEach(func() {
				configToSave = buildBasedConfig
			})

			It("succeeds", func() {
				Ω(saveErr).ShouldNot(HaveOccurred())
			})

			It("converts them to a plan before saving in the nested config db", func() {
				Ω(nestedDB.SaveConfigCallCount()).Should(Equal(1))

				name, savedConfig, savedID, savedPausedState := nestedDB.SaveConfigArgsForCall(0)
				Ω(name).Should(Equal(pipelineName))
				Ω(savedConfig).Should(Equal(planBasedConfig))
				Ω(savedID).Should(Equal(ConfigVersion(42)))
				Ω(savedPausedState).Should(Equal(PipelinePaused))
			})

			Context("when the nested config db fails to save", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					nestedDB.SaveConfigReturns(disaster)
				})

				It("returns the error", func() {
					Ω(saveErr).Should(HaveOccurred())
				})
			})
		})

		Context("when the given config contains jobs with plans", func() {
			BeforeEach(func() {
				configToSave = planBasedConfig
			})

			It("succeeds", func() {
				Ω(saveErr).ShouldNot(HaveOccurred())
			})

			It("passes them through to the nested config db", func() {
				Ω(nestedDB.SaveConfigCallCount()).Should(Equal(1))

				savedName, savedConfig, savedID, savedPausedState := nestedDB.SaveConfigArgsForCall(0)
				Ω(savedName).Should(Equal(pipelineName))
				Ω(savedConfig).Should(Equal(planBasedConfig))
				Ω(savedID).Should(Equal(ConfigVersion(42)))
				Ω(savedPausedState).Should(Equal(PipelinePaused))
			})

			Context("when the nested config db fails to save", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					nestedDB.SaveConfigReturns(disaster)
				})

				It("returns the error", func() {
					Ω(saveErr).Should(HaveOccurred())
				})
			})
		})
	})
})
