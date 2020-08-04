package atc

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"
)

const tempFilePattern = "___dash-stdin-path-flag___"

type PathFlag string

func (path *PathFlag) UnmarshalFlag(value string) error {
	if value == "" {
		return nil
	}

	if value == "-" {
		_, err := os.Stat("/dev/stdin")
		if err == nil {
			*path = PathFlag("/dev/stdin")
			return nil
		}

		tempf, err := ioutil.TempFile("", tempFilePattern)
		if err != nil {
			return fmt.Errorf("failed to create a temp file")
		}
		defer tempf.Close()

		_, err = io.Copy(tempf, os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to write temp file: %s", err.Error())
		}
		*path = PathFlag(tempf.Name())
		return nil
	}

	matches, err := filepath.Glob(value)
	if err != nil {
		return fmt.Errorf("failed to expand path '%s': %s", value, err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("path '%s' does not exist", value)
	}

	if len(matches) > 1 {
		return fmt.Errorf("path '%s' resolves to multiple entries: %s", value, strings.Join(matches, ", "))
	}

	*path = PathFlag(matches[0])
	return nil
}

func (path *PathFlag) Complete(match string) []flags.Completion {
	matches, _ := filepath.Glob(match + "*")
	comps := make([]flags.Completion, len(matches))

	for i, v := range matches {
		comps[i].Item = v
	}

	return comps
}

func (path *PathFlag) FromStdin() bool {
	return *path == "/dev/stdin" || strings.Index(string(*path), tempFilePattern) >= 0
}
