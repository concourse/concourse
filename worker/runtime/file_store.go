package runtime

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

//counterfeiter:generate . FileStore

// FileStore is responsible for managing files associated with containers.
//
type FileStore interface {
	// CreateFile creates a file with a particular content in the store.
	//
	Create(name string, content []byte) (absPath string, err error)

	// Append appends to a file previously created in the store.
	//
	Append(name string, content []byte) error

	// DeleteFile removes a file previously created in the store.
	//
	Delete(name string) (err error)
}

type fileStore struct {
	root string
}

var _ FileStore = (*fileStore)(nil)

func NewFileStore(root string) *fileStore {
	return &fileStore{
		root: root,
	}
}

func (f fileStore) Create(name string, content []byte) (string, error) {
	absPath := filepath.Join(f.root, name)
	dir := filepath.Dir(absPath)

	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return "", fmt.Errorf("mkdirall: %w", err)
	}

	err = ioutil.WriteFile(absPath, content, 0755)
	if err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return absPath, nil
}

func (f fileStore) Append(name string, content []byte) error {
	absPath := filepath.Join(f.root, name)

	file, err := os.OpenFile(absPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func (f fileStore) Delete(path string) error {
	absPath := filepath.Join(f.root, path)

	err := os.RemoveAll(absPath)
	if err != nil {
		return fmt.Errorf("remove all: %w", err)
	}

	return nil
}
