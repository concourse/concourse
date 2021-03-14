package resource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/runtime"
)

const resourceProcessID = "resource"
const resultCachePropertyName = "concourse:resource-result"

type VersionResult struct {
	Version  atc.Version         `json:"version"`
	Metadata []atc.MetadataField `json:"metadata,omitempty"`
}

type Resource struct {
	Source  atc.Source  `json:"source"`
	Params  atc.Params  `json:"params,omitempty"`
	Version atc.Version `json:"version,omitempty"`
}

func (resource Resource) Signature() ([]byte, error) {
	return json.Marshal(resource)
}

func (resource Resource) Check(ctx context.Context, container runtime.Container, stderr io.Writer) ([]atc.Version, runtime.ProcessResult, error) {
	spec := runtime.ProcessSpec{
		Path: "/opt/resource/check",
	}

	var versions []atc.Version
	processResult, err := resource.run(ctx, container, spec, stderr, false, &versions)
	if err != nil {
		return nil, runtime.ProcessResult{}, err
	}
	return versions, processResult, nil
}

func (resource Resource) Get(ctx context.Context, container runtime.Container, stderr io.Writer) (VersionResult, runtime.ProcessResult, error) {
	var versionResult VersionResult

	properties, err := container.Properties()
	if err != nil {
		return VersionResult{}, runtime.ProcessResult{}, err
	}

	if result := properties[resultCachePropertyName]; result != "" {
		if err := json.Unmarshal([]byte(result), &versionResult); err != nil {
			return VersionResult{}, runtime.ProcessResult{}, err
		}
		return versionResult, runtime.ProcessResult{}, nil
	}

	spec := runtime.ProcessSpec{
		ID:   resourceProcessID,
		Path: "/opt/resource/in",
		Args: []string{ResourcesDir("get")},
	}

	processResult, err := resource.run(ctx, container, spec, stderr, true, &versionResult)
	if err != nil {
		return VersionResult{}, runtime.ProcessResult{}, err
	}

	if err := resource.cacheResult(container, versionResult); err != nil {
		return VersionResult{}, runtime.ProcessResult{}, err
	}

	return versionResult, processResult, nil
}

func (resource Resource) Put(ctx context.Context, container runtime.Container, stderr io.Writer) (VersionResult, runtime.ProcessResult, error) {
	var versionResult VersionResult

	properties, err := container.Properties()
	if err != nil {
		return VersionResult{}, runtime.ProcessResult{}, err
	}

	if result := properties[resultCachePropertyName]; result != "" {
		if err := json.Unmarshal([]byte(result), &versionResult); err != nil {
			return VersionResult{}, runtime.ProcessResult{}, err
		}
		return versionResult, runtime.ProcessResult{}, nil
	}

	spec := runtime.ProcessSpec{
		ID:   resourceProcessID,
		Path: "/opt/resource/out",
		Args: []string{ResourcesDir("put")},
	}

	processResult, err := resource.run(ctx, container, spec, stderr, true, &versionResult)
	if err != nil {
		return VersionResult{}, runtime.ProcessResult{}, err
	}
	if processResult.ExitStatus == 0 && versionResult.Version == nil {
		return VersionResult{}, runtime.ProcessResult{}, fmt.Errorf("resource script (%s %s) output a null version", spec.Path, strings.Join(spec.Args, " "))
	}

	if err := resource.cacheResult(container, versionResult); err != nil {
		return VersionResult{}, runtime.ProcessResult{}, err
	}

	return versionResult, processResult, nil
}

func attachOrRun(ctx context.Context, container runtime.Container, spec runtime.ProcessSpec, io runtime.ProcessIO) (runtime.Process, error) {
	process, err := container.Attach(ctx, spec.ID, io)
	if err == nil {
		return process, nil
	}

	return container.Run(ctx, spec, io)
}

func (resource Resource) run(ctx context.Context, container runtime.Container, spec runtime.ProcessSpec, stderr io.Writer, attach bool, output interface{}) (runtime.ProcessResult, error) {
	input, err := resource.Signature()
	if err != nil {
		return runtime.ProcessResult{}, err
	}

	buf := new(bytes.Buffer)
	io := runtime.ProcessIO{
		Stdin:  bytes.NewBuffer(input),
		Stdout: buf,
		Stderr: stderr,
	}

	var process runtime.Process
	if attach {
		process, err = attachOrRun(ctx, container, spec, io)
	} else {
		process, err = container.Run(ctx, spec, io)
	}
	if err != nil {
		return runtime.ProcessResult{}, err
	}

	result, err := process.Wait(ctx)
	if err != nil {
		return runtime.ProcessResult{}, err
	}
	if result.ExitStatus != 0 {
		return result, nil
	}

	if err := json.Unmarshal(buf.Bytes(), output); err != nil {
		return runtime.ProcessResult{}, err
	}
	return result, nil
}

func (resource Resource) cacheResult(container runtime.Container, result VersionResult) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}

	return container.SetProperty(resultCachePropertyName, string(payload))
}

func ResourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}
