package migration

import (
	"errors"
	"os"
)

func Asset(string) ([]byte, error) {
	return nil, errors.New("no assets")
}

func MustAsset(string) []byte {
	panic("no assets")
}

func AssetInfo(string) (os.FileInfo, error) {
	return nil, errors.New("no assets")
}

func AssetNames() []string {
	return nil
}

func AssetDir(string) ([]string, error) {
	return nil, errors.New("no assets")
}

func RestoreAsset(string, string) error {
	return errors.New("no assets")
}

func RestoreAssets(string, string) error {
	return errors.New("no assets")
}
