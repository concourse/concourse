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
			Expect(nestedDB.GetConfigCallCount()).To(Equal(1))

			name := nestedDB.GetConfigArgsForCall(0)
			Expect(name).To(Equal(pipelineName))
		})

		Context("when the nested config db yields a config containing jobs with plans", func() {
			BeforeEach(func() {
				nestedDB.GetConfigReturns(planBasedConfig, 42, nil)
			})

			It("succeeds", func() {
				Expect(getErr).NotTo(HaveOccurred())
			})

			It("returns the config ID", func() {
				Expect(gotVersion).To(Equal(ConfigVersion(42)))
			})

			It("returns the config as-is", func() {
				Expect(gotConfig).To(Equal(planBasedConfig))
			})
		})

		Context("when the nested config db yields a config containing jobs with inputs/outputs/build", func() {
			BeforeEach(func() {
				nestedDB.GetConfigReturns(buildBasedConfig, 42, nil)
			})

			It("succeeds", func() {
				Expect(getErr).NotTo(HaveOccurred())
			})

			It("returns the config ID", func() {
				Expect(gotVersion).To(Equal(ConfigVersion(42)))
			})

			It("returns the config with the job converted to using plans", func() {
				Expect(gotConfig).To(Equal(planBasedConfig))
			})
		})

		Context("when the nested config db fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				nestedDB.GetConfigReturns(atc.Config{}, 0, disaster)
			})

			It("returns the error", func() {
				Expect(getErr).To(Equal(disaster))
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
			_, saveErr = configDB.SaveConfig(pipelineName, configToSave, versionToSave, pausedState)
		})

		Context("when the given config contains jobs with inputs/outputs/build", func() {
			BeforeEach(func() {
				configToSave = buildBasedConfig
			})

			It("succeeds", func() {
				Expect(saveErr).NotTo(HaveOccurred())
			})

			It("converts them to a plan before saving in the nested config db", func() {
				Expect(nestedDB.SaveConfigCallCount()).To(Equal(1))

				name, savedConfig, savedID, savedPausedState := nestedDB.SaveConfigArgsForCall(0)
				Expect(name).To(Equal(pipelineName))
				Expect(savedConfig).To(Equal(planBasedConfig))
				Expect(savedID).To(Equal(ConfigVersion(42)))
				Expect(savedPausedState).To(Equal(PipelinePaused))
			})

			Context("when the nested config db fails to save", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					nestedDB.SaveConfigReturns(false, disaster)
				})

				It("returns the error", func() {
					Expect(saveErr).To(HaveOccurred())
				})
			})
		})

		Context("when the given config contains jobs with plans", func() {
			BeforeEach(func() {
				configToSave = planBasedConfig
			})

			It("succeeds", func() {
				Expect(saveErr).NotTo(HaveOccurred())
			})

			It("passes them through to the nested config db", func() {
				Expect(nestedDB.SaveConfigCallCount()).To(Equal(1))

				savedName, savedConfig, savedID, savedPausedState := nestedDB.SaveConfigArgsForCall(0)
				Expect(savedName).To(Equal(pipelineName))
				Expect(savedConfig).To(Equal(planBasedConfig))
				Expect(savedID).To(Equal(ConfigVersion(42)))
				Expect(savedPausedState).To(Equal(PipelinePaused))
			})

			Context("when the nested config db fails to save", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					nestedDB.SaveConfigReturns(false, disaster)
				})

				It("returns the error", func() {
					Expect(saveErr).To(HaveOccurred())
				})
			})
		})
	})
})
