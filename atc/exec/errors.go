package exec

import "fmt"

// FileNotFoundError is the error to return from StreamFile when the given path
// does not exist.
type FileNotFoundError struct {
	Path string
}

// Error prints a helpful message including the file path. The user will see
// this message if e.g. their task config path does not exist.
func (err FileNotFoundError) Error() string {
	return fmt.Sprintf("file not found: %s", err.Path)
}
