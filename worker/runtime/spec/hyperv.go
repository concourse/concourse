package spec

import (
	"maps"
	"strconv"

	"github.com/Microsoft/hcsshim/pkg/annotations"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// UtilityVM sizes the utility VM that a Hyper-V isolated container boots in.
// A zero field leaves that size to the runtime's default.
type UtilityVM struct {
	ProcessorCount uint
	MemoryMB       uint
}

// Isolate makes a Windows spec run in its own utility VM rather than share
// the host's kernel.
func (u UtilityVM) Isolate(oci *specs.Spec) {
	oci.Windows.HyperV = &specs.WindowsHyperV{}

	a := maps.Clone(oci.Annotations)
	if a == nil {
		a = map[string]string{}
	}

	if u.ProcessorCount != 0 {
		a[annotations.ProcessorCount] = strconv.FormatUint(uint64(u.ProcessorCount), 10)
	}

	// the annotation would override the size the runtime takes from the
	// container's own memory limit
	res := oci.Windows.Resources
	if u.MemoryMB != 0 && (res == nil || res.Memory == nil || res.Memory.Limit == nil) {
		a[annotations.MemorySizeInMB] = strconv.FormatUint(uint64(u.MemoryMB), 10)
	}

	oci.Annotations = a
}
