package flag

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type File string

func (f File) MarshalYAML() (interface{}, error) {
	return string(f), nil
}

func (f *File) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	err := unmarshal(&value)
	if err != nil {
		return err
	}

	if value != "" {
		return f.Set(value)
	}

	return nil
}

// Can be removed once flags are deprecated
func (f *File) Set(value string) error {
	stat, err := os.Stat(value)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		return fmt.Errorf("path '%s' is a directory, not a file", value)
	}

	abs, err := filepath.Abs(value)
	if err != nil {
		return err
	}

	*f = File(abs)

	return nil
}

// Can be removed once flags are deprecated
func (f *File) String() string {
	return string(*f)
}

// Can be removed once flags are deprecated
func (f *File) Type() string {
	return "File"
}

func (f *File) Path() string {
	return string(*f)
}

type Files []File

// Can be removed once flags are deprecated
func (f *Files) Set(value string) error {
	unparsedFiles := strings.Split(value, ",")

	var parsedFiles Files
	for _, unparsedFile := range unparsedFiles {
		var file File
		err := file.Set(strings.TrimSpace(unparsedFile))
		if err != nil {
			return err
		}

		parsedFiles = append(parsedFiles, file)
	}

	return nil
}

// Can be removed once flags are deprecated
func (f *Files) String() string {
	return fmt.Sprintf("%v", *f)
}

// Can be removed once flags are deprecated
func (f *Files) Type() string {
	return "Files"
}
