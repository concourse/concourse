package migration

import (
	"github.com/gobuffalo/packr"
)

//go:generate counterfeiter . Bindata

type Bindata interface {
	AssetNames() []string
	Asset(name string) ([]byte, error)
}

type packrSource struct {
	packr.Box
}

func (bs *packrSource) AssetNames() []string {
	migrations := []string{}
	for _, name := range bs.Box.List() {
		if name != "migrations.go" {
			migrations = append(migrations, name)
		}
	}

	return migrations
}

func (bs *packrSource) Asset(name string) ([]byte, error) {
	return bs.Box.MustBytes(name)
}
