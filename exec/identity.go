package exec

import "os"

type Identity struct{}

func (Identity) Using(source ArtifactSource) ArtifactSource {
	return identitySource{source}
}

type identitySource struct {
	ArtifactSource
}

func (identitySource) Run(<-chan os.Signal, chan<- struct{}) error {
	return nil
}
