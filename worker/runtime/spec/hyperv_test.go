package spec_test

import (
	"maps"
	"testing"

	"github.com/concourse/concourse/v8/worker/runtime/spec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func TestUtilityVMIsolate(t *testing.T) {
	const (
		processorCount = "io.microsoft.virtualmachine.computetopology.processor.count"
		memorySize     = "io.microsoft.virtualmachine.computetopology.memory.sizeinmb"
	)

	memoryLimit := uint64(512 * 1024 * 1024)

	for _, tc := range []struct {
		name        string
		uvm         spec.UtilityVM
		limits      *specs.WindowsResources
		properties  map[string]string
		annotations map[string]string
	}{
		{
			name: "zero value leaves sizing to the runtime",
		},
		{
			name:        "sizes the utility VM",
			uvm:         spec.UtilityVM{ProcessorCount: 4, MemoryMB: 4096},
			annotations: map[string]string{processorCount: "4", memorySize: "4096"},
		},
		{
			name:        "container memory limit wins over the worker's memory size",
			uvm:         spec.UtilityVM{ProcessorCount: 4, MemoryMB: 4096},
			limits:      &specs.WindowsResources{Memory: &specs.WindowsMemoryResources{Limit: &memoryLimit}},
			annotations: map[string]string{processorCount: "4"},
		},
		{
			name:        "keeps garden properties",
			uvm:         spec.UtilityVM{MemoryMB: 4096},
			properties:  map[string]string{"user": "property"},
			annotations: map[string]string{"user": "property", memorySize: "4096"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			properties := maps.Clone(tc.properties)
			oci := &specs.Spec{
				Annotations: tc.properties,
				Windows:     &specs.Windows{Resources: tc.limits},
			}

			tc.uvm.Isolate(oci)

			if oci.Windows.HyperV == nil {
				t.Error("expected Hyper-V isolation")
			}

			if !maps.Equal(oci.Annotations, tc.annotations) {
				t.Errorf("unexpected annotations: %v", oci.Annotations)
			}

			if !maps.Equal(tc.properties, properties) {
				t.Errorf("garden properties changed: %v", tc.properties)
			}
		})
	}
}
