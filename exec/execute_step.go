package exec

import (
	"archive/tar"
	"errors"
	"io"
	"os"
	"path"
	"strings"

	garden "github.com/cloudfoundry-incubator/garden/api"
	"github.com/concourse/atc"
)

const ArtifactsRoot = "/tmp/build/src"

var ErrInterrupted = errors.New("interrupted")

type executeStep struct {
	IOConfig IOConfig

	GardenClient garden.Client
	ConfigSource BuildConfigSource

	ArtifactSource ArtifactSource

	Container garden.Container

	successful bool
}

func (step executeStep) Using(source ArtifactSource) ArtifactSource {
	step.ArtifactSource = source
	return &step
}

func (step *executeStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	config, err := step.ConfigSource.FetchConfig(step.ArtifactSource)
	if err != nil {
		return err
	}

	step.Container, err = step.GardenClient.Create(garden.ContainerSpec{
		RootFSPath: config.Image,
	})
	if err != nil {
		return err
	}

	err = step.ArtifactSource.StreamTo(containerDestination{
		Container:    step.Container,
		InputConfigs: config.Inputs,
	})
	if err != nil {
		return err
	}

	process, err := step.Container.Run(garden.ProcessSpec{
		Path: config.Run.Path,
		Args: config.Run.Args,
		Env:  step.envForParams(config.Params),

		Dir: ArtifactsRoot,
	}, garden.ProcessIO{
		Stdout: step.IOConfig.Stdout,
		Stderr: step.IOConfig.Stderr,
	})
	if err != nil {
		return err
	}

	close(ready)

	waitExitStatus := make(chan int, 1)
	waitErr := make(chan error, 1)
	go func() {
		status, err := process.Wait()
		if err != nil {
			waitErr <- err
		} else {
			waitExitStatus <- status
		}
	}()

	select {
	case <-signals:
		step.Container.Stop(false)
		return ErrInterrupted

	case status := <-waitExitStatus:
		step.successful = status == 0
		return nil

	case err := <-waitErr:
		return err
	}
}

func (step *executeStep) Successful() bool {
	return step.successful
}

func (step *executeStep) Release() error {
	return step.GardenClient.Destroy(step.Container.Handle())
}

func (step *executeStep) StreamFile(source string) (io.ReadCloser, error) {
	out, err := step.Container.StreamOut(path.Join(ArtifactsRoot, source))
	if err != nil {
		return nil, err
	}

	tarReader := tar.NewReader(out)

	_, err = tarReader.Next()
	if err != nil {
		return nil, ErrFileNotFound
	}

	return fileReadCloser{
		Reader: tarReader,
		Closer: out,
	}, nil
}

func (step *executeStep) StreamTo(destination ArtifactDestination) error {
	out, err := step.Container.StreamOut(ArtifactsRoot)
	if err != nil {
		return err
	}

	return destination.StreamIn(".", out)
}

func (executeStep) envForParams(params map[string]string) []string {
	env := make([]string, 0, len(params))

	for k, v := range params {
		env = append(env, k+"="+v)
	}

	return env
}

type containerDestination struct {
	Container garden.Container

	InputConfigs []atc.BuildInputConfig
}

func (dest containerDestination) StreamIn(dst string, src io.Reader) error {
	destSegments := strings.Split(dst, "/")

	if len(destSegments) > 0 {
		for _, config := range dest.InputConfigs {
			if config.Name == destSegments[0] {
				destSegments[0] = config.Path
				break
			}
		}
	}

	return dest.Container.StreamIn(path.Join(ArtifactsRoot, strings.Join(destSegments, "/")), src)
}
