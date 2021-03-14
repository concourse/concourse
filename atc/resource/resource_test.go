package resource

import (
	"bytes"
	"context"
	"testing"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	"github.com/stretchr/testify/require"
)

func TestResourceCheck(t *testing.T) {
	resource := Resource{
		Source:  atc.Source{"some": "source"},
		Params:  atc.Params{"some": "params"},
		Version: atc.Version{"some": "version"},
	}
	ctx := context.Background()
	expectedSpec := runtime.ProcessSpec{
		Path: "/opt/resource/check",
	}

	t.Run("successful run", func(t *testing.T) {
		expectedVersions := []atc.Version{
			{"version": "v1"},
			{"version": "v2"},
		}
		container := runtimetest.NewContainer().
			WithProcess(
				expectedSpec,
				runtimetest.ProcessStub{
					Output: expectedVersions,
					Stderr: "some stderr log",
				},
			)
		stderr := new(bytes.Buffer)
		versions, processResult, err := resource.Check(ctx, container, stderr)
		require.NoError(t, err)
		require.Equal(t, expectedVersions, versions)
		require.Equal(t, 0, processResult.ExitStatus)

		require.Equal(t, "some stderr log", stderr.String())
	})

	t.Run("error", func(t *testing.T) {
		container := runtimetest.NewContainer().
			WithProcess(
				expectedSpec,
				runtimetest.ProcessStub{
					Err: "failed",
				},
			)
		_, _, err := resource.Check(ctx, container, new(bytes.Buffer))
		require.Error(t, err)
	})

	t.Run("non-zero exit status", func(t *testing.T) {
		container := runtimetest.NewContainer().
			WithProcess(
				expectedSpec,
				runtimetest.ProcessStub{
					ExitStatus: 123,
				},
			)
		_, processResult, err := resource.Check(ctx, container, new(bytes.Buffer))
		require.NoError(t, err)
		require.Equal(t, 123, processResult.ExitStatus)
	})
}

func TestResourceGet(t *testing.T) {
	resource := Resource{
		Source:  atc.Source{"some": "source"},
		Params:  atc.Params{"some": "params"},
		Version: atc.Version{"some": "version"},
	}
	ctx := context.Background()
	expectedSpec := runtime.ProcessSpec{
		ID:   "resource",
		Path: "/opt/resource/in",
		Args: []string{"/tmp/build/get"},
	}

	t.Run("successful run", func(t *testing.T) {
		expectedResult := VersionResult{
			Version: atc.Version{"version": "v1"},
		}
		container := runtimetest.NewContainer().
			WithProcess(
				expectedSpec,
				runtimetest.ProcessStub{
					Output: expectedResult,
					Stderr: "some stderr log",
				},
			)
		stderr := new(bytes.Buffer)
		result, processResult, err := resource.Get(ctx, container, stderr)
		require.NoError(t, err)
		require.Equal(t, expectedResult, result)
		require.Equal(t, 0, processResult.ExitStatus)

		require.Equal(t, "some stderr log", stderr.String())
	})

	t.Run("successful attach", func(t *testing.T) {
		expectedResult := VersionResult{
			Version: atc.Version{"version": "v1"},
		}
		container := runtimetest.NewContainer().
			WithProcess(
				expectedSpec,
				runtimetest.ProcessStub{
					Attachable: true,
					Output:     expectedResult,
				},
			)
		result, processResult, err := resource.Get(ctx, container, new(bytes.Buffer))
		require.NoError(t, err)
		require.Equal(t, expectedResult, result)
		require.Equal(t, 0, processResult.ExitStatus)
	})

	t.Run("caches the result", func(t *testing.T) {
		expectedResult := VersionResult{
			Version: atc.Version{"version": "v1"},
		}
		container := runtimetest.NewContainer().
			WithProcess(
				expectedSpec,
				runtimetest.ProcessStub{
					Output: expectedResult,
				},
			)
		result, processResult, err := resource.Get(ctx, container, new(bytes.Buffer))
		require.NoError(t, err)
		require.Equal(t, expectedResult, result)
		require.Equal(t, 0, processResult.ExitStatus)

		// overwrite the process' return value if called again
		container = container.WithProcess(
			expectedSpec,
			runtimetest.ProcessStub{
				Output: VersionResult{
					Version: atc.Version{"version": "other-version"},
				},
			},
		)
		result, processResult, err = resource.Get(ctx, container, new(bytes.Buffer))
		require.NoError(t, err)
		// validate the process didn't get called again - the result was cached
		require.Equal(t, expectedResult, result)
		require.Equal(t, 0, processResult.ExitStatus)

	})

	t.Run("error", func(t *testing.T) {
		container := runtimetest.NewContainer().
			WithProcess(
				expectedSpec,
				runtimetest.ProcessStub{
					Err: "failed",
				},
			)
		_, _, err := resource.Get(ctx, container, new(bytes.Buffer))
		require.Error(t, err)
	})

	t.Run("non-zero exit status", func(t *testing.T) {
		t.Run("attach", func(t *testing.T) {
			container := runtimetest.NewContainer().
				WithProcess(
					expectedSpec,
					runtimetest.ProcessStub{
						Attachable: true,
						ExitStatus: 123,
					},
				)
			_, processResult, err := resource.Get(ctx, container, new(bytes.Buffer))
			require.NoError(t, err)
			require.Equal(t, 123, processResult.ExitStatus)
		})

		t.Run("run", func(t *testing.T) {
			container := runtimetest.NewContainer().
				WithProcess(
					expectedSpec,
					runtimetest.ProcessStub{
						Attachable: false,
						ExitStatus: 123,
					},
				)
			_, processResult, err := resource.Get(ctx, container, new(bytes.Buffer))
			require.NoError(t, err)
			require.Equal(t, 123, processResult.ExitStatus)
		})
	})
}

func TestResourcePut(t *testing.T) {
	resource := Resource{
		Source:  atc.Source{"some": "source"},
		Params:  atc.Params{"some": "params"},
		Version: atc.Version{"some": "version"},
	}
	ctx := context.Background()
	expectedSpec := runtime.ProcessSpec{
		ID:   resourceProcessID,
		Path: "/opt/resource/out",
		Args: []string{"/tmp/build/put"},
	}

	t.Run("successful run", func(t *testing.T) {
		expectedResult := VersionResult{
			Version: atc.Version{"version": "v1"},
		}
		container := runtimetest.NewContainer().
			WithProcess(
				expectedSpec,
				runtimetest.ProcessStub{
					Output: expectedResult,
					Stderr: "some stderr log",
				},
			)
		stderr := new(bytes.Buffer)
		result, processResult, err := resource.Put(ctx, container, stderr)
		require.NoError(t, err)
		require.Equal(t, expectedResult, result)
		require.Equal(t, 0, processResult.ExitStatus)

		require.Equal(t, "some stderr log", stderr.String())
	})

	t.Run("successful attach", func(t *testing.T) {
		expectedResult := VersionResult{
			Version: atc.Version{"version": "v1"},
		}
		container := runtimetest.NewContainer().
			WithProcess(
				expectedSpec,
				runtimetest.ProcessStub{
					Attachable: true,
					Output:     expectedResult,
				},
			)
		result, processResult, err := resource.Put(ctx, container, new(bytes.Buffer))
		require.NoError(t, err)
		require.Equal(t, expectedResult, result)
		require.Equal(t, 0, processResult.ExitStatus)
	})

	t.Run("caches the result", func(t *testing.T) {
		expectedResult := VersionResult{
			Version: atc.Version{"version": "v1"},
		}
		container := runtimetest.NewContainer().
			WithProcess(
				expectedSpec,
				runtimetest.ProcessStub{
					Output: expectedResult,
				},
			)
		result, processResult, err := resource.Put(ctx, container, new(bytes.Buffer))
		require.NoError(t, err)
		require.Equal(t, expectedResult, result)
		require.Equal(t, 0, processResult.ExitStatus)

		// overwrite the process' return value if called again
		container = container.WithProcess(
			expectedSpec,
			runtimetest.ProcessStub{
				Output: VersionResult{
					Version: atc.Version{"version": "other-version"},
				},
			},
		)
		result, processResult, err = resource.Put(ctx, container, new(bytes.Buffer))
		require.NoError(t, err)
		// validate the process didn't get called again - the result was cached
		require.Equal(t, expectedResult, result)
		require.Equal(t, 0, processResult.ExitStatus)

	})

	t.Run("nil version", func(t *testing.T) {
		container := runtimetest.NewContainer().
			WithProcess(
				expectedSpec,
				runtimetest.ProcessStub{
					Output: VersionResult{},
				},
			)
		_, _, err := resource.Put(ctx, container, new(bytes.Buffer))
		require.Error(t, err)
	})

	t.Run("error", func(t *testing.T) {
		container := runtimetest.NewContainer().
			WithProcess(
				expectedSpec,
				runtimetest.ProcessStub{
					Err: "failed",
				},
			)
		_, _, err := resource.Put(ctx, container, new(bytes.Buffer))
		require.Error(t, err)
	})

	t.Run("non-zero exit status", func(t *testing.T) {
		t.Run("attach", func(t *testing.T) {
			container := runtimetest.NewContainer().
				WithProcess(
					expectedSpec,
					runtimetest.ProcessStub{
						Attachable: true,
						ExitStatus: 123,
					},
				)
			_, processResult, err := resource.Put(ctx, container, new(bytes.Buffer))
			require.NoError(t, err)
			require.Equal(t, 123, processResult.ExitStatus)
		})

		t.Run("run", func(t *testing.T) {
			container := runtimetest.NewContainer().
				WithProcess(
					expectedSpec,
					runtimetest.ProcessStub{
						Attachable: false,
						ExitStatus: 123,
					},
				)
			_, processResult, err := resource.Put(ctx, container, new(bytes.Buffer))
			require.NoError(t, err)
			require.Equal(t, 123, processResult.ExitStatus)
		})
	})
}
