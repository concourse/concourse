package atc

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"
)

type PathFlag string

func (path *PathFlag) UnmarshalFlag(value string) error {
	if value == "" {
		return nil
	}

	if isURL(value) {
		*path = PathFlag(value)
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
	var matches []string
	if isURL(match) {
		matches = []string{}
	} else {
		matches, _ = filepath.Glob(match + "*")
	}
	comps := make([]flags.Completion, len(matches))

	for i, v := range matches {
		comps[i].Item = v
	}

	return comps
}

func (path *PathFlag) ReadContent() ([]byte, error) {
	var (
		content []byte
		err     error
	)
	if path.IsURL() {
		res, err := http.Get(string(*path))
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%d: %s", res.StatusCode, http.StatusText(res.StatusCode))
		}
		content, err = ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
	} else {
		content, err = ioutil.ReadFile(string(*path))
		if err != nil {
			return nil, err
		}
	}
	return content, nil
}

func (path *PathFlag) IsURL() bool {
	return isURL(string(*path))
}

func isURL(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}
