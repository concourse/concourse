package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/txn2/txeh"
)

//counterfeiter:generate . FileStore

// FileStore is responsible for managing files associated with containers.
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

	// ContainerIpLookup find container IP by container handle in hosts file
	ContainerIpLookup(handle string) (string, error)
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

	err = os.WriteFile(absPath, content, 0755)
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

func (f fileStore) ContainerIpLookup(handle string) (string, error) {
	absPath := filepath.Join(f.root, handle)

	hc := txeh.HostsConfig{ReadFilePath: filepath.Join(absPath, "/hosts")}
	hosts, err := txeh.NewHosts(&hc)
	if err != nil {
		return "", fmt.Errorf("error reading hosts file: %w", err)
	}

	found, ip, _ := hosts.HostAddressLookup(handle, txeh.IPFamilyV4)
	if !found {
		return "", fmt.Errorf("ip not found for container handle: %s", handle)
	}
	if err != nil {
		return "", fmt.Errorf("error finding container ip: %w", err)
	}

	return ip, nil
}
