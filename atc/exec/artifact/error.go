package artifact

import (
	"fmt"
)

// UnspecifiedArtifactSourceError is returned when the specified path is of a
// file in the toplevel directory, and so it does not indicate a SourceName.
type UnspecifiedArtifactSourceError struct {
	Path string
}

// Error returns a human-friendly error message.
func (err UnspecifiedArtifactSourceError) Error() string {
	return fmt.Sprintf("path '%s' does not specify where the file lives", err.Path)
}

// UnknownArtifactSourceError is returned when the artifact.Name specified by the
// path does not exist in the artifact.Repository.
type UnknownArtifactSourceError struct {
	Name string
	Path string
}

// Error returns a human-friendly error message.
func (err UnknownArtifactSourceError) Error() string {
	return fmt.Sprintf("unknown artifact source: '%s' in file path '%s'", err.Name, err.Path)
}

// FileNotFoundError is returned when the specified file path does not
// exist within its artifact source.
type FileNotFoundError struct {
	Name     string
	FilePath string
}

// Error returns a human-friendly error message.
func (err FileNotFoundError) Error() string {
	return fmt.Sprintf("file '%s' not found within artifact '%s'", err.FilePath, err.Name)
}
