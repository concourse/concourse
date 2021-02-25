package gardenruntimetest

import (
	"encoding/json"
	"reflect"
	"testing/fstest"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/worker2/gardenruntime"
	. "github.com/onsi/gomega"
)

func ImageMetadataFile(metadata gardenruntime.ImageMetadata) *fstest.MapFile {
	data, err := json.Marshal(metadata)
	Expect(err).ToNot(HaveOccurred())
	return &fstest.MapFile{Data: data}
}

func StrategyEq(strategy baggageclaim.Strategy) func(*Volume) bool {
	return func(v *Volume) bool {
		return reflect.DeepEqual(v.Spec.Strategy.Encode(), strategy.Encode())
	}
}

func PrivilegedEq(p bool) func(*Volume) bool {
	return func(v *Volume) bool {
		return v.Spec.Privileged == p
	}
}

func ContentEq(content fstest.MapFS) func(*Volume) bool {
	return func(v *Volume) bool {
		return reflect.DeepEqual(v.Content, content)
	}
}

func HandleEq(handle string) func(*Volume) bool {
	return func(v *Volume) bool {
		return v.Handle() == handle
	}
}

func PathEq(path string) func(*Volume) bool {
	return func(v *Volume) bool {
		return v.Path() == path
	}
}
