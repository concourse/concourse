package atc_test

import (
	. "github.com/concourse/concourse/atc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TaskConfig", func() {
	Describe("validating", func() {
		var (
			invalidConfig TaskConfig
			validConfig   TaskConfig
		)

		BeforeEach(func() {
			validConfig = TaskConfig{
				Platform: "linux",
				Run: TaskRunConfig{
					Path: "reboot",
				},
			}

			invalidConfig = validConfig
		})

		Describe("decode task yaml", func() {
			Context("given a valid task config", func() {
				It("works", func() {
					data := []byte(`
platform: beos

inputs: []

run: {path: a/file}
`)
					task, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(task.Platform).To(Equal("beos"))
					Expect(task.Run.Path).To(Equal("a/file"))
				})

				It("converts yaml booleans to strings in params", func() {
					data := []byte(`
platform: beos

params:
  testParam: true

run: {path: a/file}
`)
					config, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Params["testParam"]).To(Equal("true"))
				})

				It("converts yaml ints to the correct string in params", func() {
					data := []byte(`
platform: beos

params:
  testParam: 1059262

run: {path: a/file}
`)
					config, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Params["testParam"]).To(Equal("1059262"))
				})

				It("converts large yaml ints to the correct string in params", func() {
					data := []byte(`
platform: beos

params:
  testParam: 18446744073709551615

run: {path: a/file}
`)
					config, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Params["testParam"]).To(Equal("18446744073709551615"))
				})

				It("does not preserve unquoted float notation", func() {
					data := []byte(`
platform: beos

params:
  testParam: 1.8446744e+19

run: {path: a/file}
`)
					config, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Params["testParam"]).To(Equal("18446744000000000000"))
				})

				It("(obviously) preserves quoted float notation", func() {
					data := []byte(`
platform: beos

params:
  testParam: "1.8446744e+19"

run: {path: a/file}
`)
					config, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Params["testParam"]).To(Equal("1.8446744e+19"))
				})

				It("converts yaml floats to the correct string in params", func() {
					data := []byte(`
platform: beos

params:
  testParam: 1059262.123123123

run: {path: a/file}
`)
					config, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Params["testParam"]).To(Equal("1059262.123123123"))
				})

				It("converts maps to json in params", func() {
					data := []byte(`
platform: beos

params:
  testParam:
    foo: bar

run: {path: a/file}
`)
					config, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Params["testParam"]).To(Equal(`{"foo":"bar"}`))
				})

				It("converts empty values to empty string in params", func() {
					data := []byte(`
platform: beos

params:
  testParam:

run: {path: a/file}
`)
					config, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Params["testParam"]).To(Equal(""))
				})
			})

			Context("given a valid task config with numeric params", func() {
				It("works", func() {
					data := []byte(`
platform: beos

params:
  FOO: 1

run: {path: a/file}
`)
					task, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(task.Platform).To(Equal("beos"))
					Expect(task.Params).To(Equal(TaskEnv{"FOO": "1"}))
				})
			})

			Context("given a valid task config with extra keys", func() {
				It("returns an error", func() {
					data := []byte(`
platform: beos

intputs: []

run: {path: a/file}
`)
					_, err := NewTaskConfig(data)
					Expect(err).To(HaveOccurred())
				})
			})

			Context("given an invalid task config", func() {
				It("errors on validation", func() {
					data := []byte(`
platform: beos

inputs: ['a/b/c']
outputs: ['a/b/c']

run: {path: a/file}
`)
					_, err := NewTaskConfig(data)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when platform is missing", func() {
			BeforeEach(func() {
				invalidConfig.Platform = ""
			})

			It("returns an error", func() {
				Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("missing 'platform'")))
			})
		})

		Context("when container limits are specified", func() {
			Context("when memory and cpu limits are correctly specified", func() {
				It("successfully parses the limits with memory units", func() {
					data := []byte(`
platform: beos
container_limits: { cpu: 1024, memory: 1KB }

run: {path: a/file}
`)
					task, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					cpu := CPULimit(1024)
					memory := MemoryLimit(1024)
					Expect(task.Limits).To(Equal(&ContainerLimits{
						CPU:    &cpu,
						Memory: &memory,
					}))
				})

				It("successfully parses the limits without memory units", func() {
					data := []byte(`
platform: beos
container_limits: { cpu: 1024, memory: 209715200 }

run: {path: a/file}
`)
					task, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					cpu := CPULimit(1024)
					memory := MemoryLimit(209715200)
					Expect(task.Limits).To(Equal(&ContainerLimits{
						CPU:    &cpu,
						Memory: &memory,
					}))
				})
			})

			Context("when either one of memory or cpu is correctly specified", func() {
				It("parses the provided memory limit without any errors", func() {
					data := []byte(`
platform: beos
container_limits: { memory: 1KB }

run: {path: a/file}
`)
					task, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					memory := MemoryLimit(1024)
					Expect(task.Limits).To(Equal(&ContainerLimits{
						Memory: &memory,
					}))
				})

				It("parses the provided cpu limit without any errors", func() {
					data := []byte(`
platform: beos
container_limits: { cpu: 355 }

run: {path: a/file}
`)
					task, err := NewTaskConfig(data)
					Expect(err).ToNot(HaveOccurred())
					cpu := CPULimit(355)
					Expect(task.Limits).To(Equal(&ContainerLimits{
						CPU: &cpu,
					}))
				})
			})

			Context("when invalid memory limit value is provided", func() {
				It("throws an error and does not continue", func() {
					data := []byte(`
platform: beos
container_limits: { cpu: 1024, memory: abc1000kb  }

run: {path: a/file}
`)
					_, err := NewTaskConfig(data)
					Expect(err).To(MatchError(ContainSubstring("could not parse container memory limit")))
				})

			})

			Context("when invalid cpu limit value is provided", func() {
				It("throws an error and does not continue", func() {
					data := []byte(`
platform: beos
container_limits: { cpu: str1ng-cpu-l1mit, memory: 20MB}

run: {path: a/file}
`)
					_, err := NewTaskConfig(data)
					Expect(err).To(MatchError(ContainSubstring("cpu limit must be an integer")))
				})
			})
		})

		Context("when the task has inputs", func() {
			BeforeEach(func() {
				validConfig.Inputs = append(validConfig.Inputs, TaskInputConfig{Name: "concourse"})
			})

			It("is valid", func() {
				Expect(validConfig.Validate()).ToNot(HaveOccurred())
			})

			Context("when input.name is missing", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(invalidConfig.Inputs, TaskInputConfig{Name: "concourse"}, TaskInputConfig{Name: ""})
				})

				It("returns an error", func() {
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("input in position 1 is missing a name")))
				})
			})

			Context("when input.name is missing multiple times", func() {
				BeforeEach(func() {
					invalidConfig.Inputs = append(
						invalidConfig.Inputs,
						TaskInputConfig{Name: "concourse"},
						TaskInputConfig{Name: ""},
						TaskInputConfig{Name: ""},
					)
				})

				It("returns an error", func() {
					err := invalidConfig.Validate()

					Expect(err).To(MatchError(ContainSubstring("input in position 1 is missing a name")))
					Expect(err).To(MatchError(ContainSubstring("input in position 2 is missing a name")))
				})
			})
		})

		Context("when the task has outputs", func() {
			BeforeEach(func() {
				validConfig.Outputs = append(validConfig.Outputs, TaskOutputConfig{Name: "concourse"})
			})

			It("is valid", func() {
				Expect(validConfig.Validate()).ToNot(HaveOccurred())
			})

			Context("when output.name is missing", func() {
				BeforeEach(func() {
					invalidConfig.Outputs = append(invalidConfig.Outputs, TaskOutputConfig{Name: "concourse"}, TaskOutputConfig{Name: ""})
				})

				It("returns an error", func() {
					Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("output in position 1 is missing a name")))
				})
			})

			Context("when output.name is missing multiple times", func() {
				BeforeEach(func() {
					invalidConfig.Outputs = append(
						invalidConfig.Outputs,
						TaskOutputConfig{Name: "concourse"},
						TaskOutputConfig{Name: ""},
						TaskOutputConfig{Name: ""},
					)
				})

				It("returns an error", func() {
					err := invalidConfig.Validate()

					Expect(err).To(MatchError(ContainSubstring("output in position 1 is missing a name")))
					Expect(err).To(MatchError(ContainSubstring("output in position 2 is missing a name")))
				})
			})
		})

		Context("when run is missing", func() {
			BeforeEach(func() {
				invalidConfig.Run.Path = ""
			})

			It("returns an error", func() {
				Expect(invalidConfig.Validate()).To(MatchError(ContainSubstring("missing path to executable to run")))
			})
		})

	})

})

var _ = Context("ImageResource", func() {
	var imageResource *ImageResource
	var resourceTypes ResourceTypes

	Context("ApplySourceDefaults", func() {
		BeforeEach(func() {
			resourceTypes = ResourceTypes{}
		})

		JustBeforeEach(func() {
			imageResource.ApplySourceDefaults(resourceTypes)
		})

		Context("when imageResource is nil", func() {
			It("should not fail", func() {
				Expect(imageResource).To(BeNil())
			})
		})

		Context("when imageResource is initialized", func() {
			BeforeEach(func() {
				imageResource = &ImageResource{
					Type: "docker",
					Source: Source{
						"a":               "b",
						"evaluated-value": "((task-variable-name))",
					},
				}
			})

			Context("resourceTypes is empty, and no base resource type defaults configured", func() {
				It("applied source should be identical to the original", func() {
					Expect(imageResource.Source).To(Equal(Source{
						"a":               "b",
						"evaluated-value": "((task-variable-name))",
					}))
				})
			})

			Context("resourceTypes is empty, and base resource type defaults configured", func() {
				BeforeEach(func() {
					LoadBaseResourceTypeDefaults(map[string]Source{"docker": {"some-key": "some-value"}})
				})
				AfterEach(func() {
					LoadBaseResourceTypeDefaults(map[string]Source{})
				})

				It("defaults should be added to image source", func() {
					Expect(imageResource.Source).To(Equal(Source{
						"a":               "b",
						"evaluated-value": "((task-variable-name))",
						"some-key":        "some-value",
					}))
				})
			})

			Context("resourceTypes contains image source type", func() {
				BeforeEach(func() {
					resourceTypes = ResourceTypes{
						ResourceType{
							Name:     "docker",
							Defaults: Source{"some-key": "some-value"},
						},
					}
				})

				It("defaults should be added to image source", func() {
					Expect(imageResource.Source).To(Equal(Source{
						"a":               "b",
						"evaluated-value": "((task-variable-name))",
						"some-key":        "some-value",
					}))
				})
			})
		})
	})
})
