package exec

import (
	"io"
	"os"

	"github.com/concourse/atc"
)

type Conditional struct {
	Conditions atc.OutputConditions
	Step       Step

	inputSource  ArtifactSource
	outputSource ArtifactSource
}

func (c Conditional) Using(source ArtifactSource) ArtifactSource {
	c.inputSource = source
	return &c
}

func (c *Conditional) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	indicator, ok := c.inputSource.(SuccessIndicator)

	if ok && c.Conditions.SatisfiedBy(indicator.Successful()) {
		c.outputSource = c.Step.Using(c.inputSource)
	} else {
		c.outputSource = &NoopArtifactSource{}
	}

	return c.outputSource.Run(signals, ready)
}

func (c *Conditional) Release() error {
	return c.outputSource.Release()
}

func (c *Conditional) StreamTo(dst ArtifactDestination) error {
	return c.outputSource.StreamTo(dst)
}

func (c *Conditional) StreamFile(path string) (io.ReadCloser, error) {
	return c.outputSource.StreamFile(path)
}
