package image

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/concourse/atc/worker"
)

type MalformedMetadataError struct {
	UnmarshalError error
}

func (err MalformedMetadataError) Error() string {
	return fmt.Sprintf("malformed image metadata: %s", err.UnmarshalError)
}

func loadMetadata(tarReader io.ReadCloser) (worker.ImageMetadata, error) {
	defer tarReader.Close()

	var imageMetadata worker.ImageMetadata
	if err := json.NewDecoder(tarReader).Decode(&imageMetadata); err != nil {
		return worker.ImageMetadata{}, MalformedMetadataError{
			UnmarshalError: err,
		}
	}

	return imageMetadata, nil
}
