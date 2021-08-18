package volume

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type VolumeState string

const (
	propertiesFileName   = "properties.json"
	isPrivilegedFileName = "privileged.json"
)

type Metadata struct {
	path string
}

// Properties File
func (md *Metadata) Properties() (Properties, error) {
	return md.propertiesFile().Properties()
}

func (md *Metadata) StoreProperties(properties Properties) error {
	return md.propertiesFile().WriteProperties(properties)
}

func (md *Metadata) propertiesFile() *propertiesFile {
	return &propertiesFile{path: filepath.Join(md.path, propertiesFileName)}
}

type propertiesFile struct {
	path string
}

func (pf *propertiesFile) WriteProperties(properties Properties) error {
	return writeMetadataFile(pf.path, properties)
}

func (pf *propertiesFile) Properties() (Properties, error) {
	var properties Properties

	err := readMetadataFile(pf.path, &properties)
	if err != nil {
		return Properties{}, err
	}

	return properties, nil
}

func (md *Metadata) isPrivilegedFile() *isPrivilegedFile {
	return &isPrivilegedFile{path: filepath.Join(md.path, isPrivilegedFileName)}
}

func (md *Metadata) IsPrivileged() (bool, error) {
	return md.isPrivilegedFile().IsPrivileged()
}

func (md *Metadata) StorePrivileged(isPrivileged bool) error {
	return md.isPrivilegedFile().WritePrivileged(isPrivileged)
}

type isPrivilegedFile struct {
	path string
}

func (ipf *isPrivilegedFile) WritePrivileged(isPrivileged bool) error {
	return writeMetadataFile(ipf.path, isPrivileged)
}

func (ipf *isPrivilegedFile) IsPrivileged() (bool, error) {
	var isPrivileged bool

	err := readMetadataFile(ipf.path, &isPrivileged)
	if err != nil {
		return false, err
	}

	return isPrivileged, nil
}

func readMetadataFile(path string, properties interface{}) error {
	file, err := os.Open(path)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return ErrVolumeDoesNotExist
		}

		return err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&properties); err != nil {
		return err
	}

	return nil
}

func writeMetadataFile(path string, properties interface{}) error {
	file, err := os.OpenFile(
		path,
		os.O_WRONLY|os.O_CREATE,
		0644,
	)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return ErrVolumeDoesNotExist
		}

		return err
	}

	defer file.Close()

	return json.NewEncoder(file).Encode(properties)
}
