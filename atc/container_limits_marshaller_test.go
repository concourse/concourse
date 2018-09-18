package atc_test

import (
	"encoding/json"

	. "github.com/concourse/atc"
	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerLimits", func() {
	Context("when unmarshaling a container_limits from YAML", func() {
		It("produces the correct ContainerLimits object without error", func() {
			var containerLimits ContainerLimits
			bs := []byte(`{ cpu: 1024, memory: 1024 }`)
			err := yaml.Unmarshal(bs, &containerLimits)
			Expect(err).NotTo(HaveOccurred())

			cpu := uint64(1024)
			mem := uint64(1024)
			expected := ContainerLimits{
				CPU:    &cpu,
				Memory: &mem,
			}

			Expect(containerLimits).To(Equal(expected))
		})
	})

	Context("when unmarshaling a container_limits from JSON", func() {
		It("produces the correct ContainerLimits without error", func() {
			var containerLimits ContainerLimits
			bs := []byte(`{ "cpu": 1024, "memory": 1024 }`)
			err := json.Unmarshal(bs, &containerLimits)
			Expect(err).NotTo(HaveOccurred())

			cpu := uint64(1024)
			mem := uint64(1024)
			expected := ContainerLimits{
				CPU:    &cpu,
				Memory: &mem,
			}

			Expect(containerLimits).To(Equal(expected))
		})
	})
})
