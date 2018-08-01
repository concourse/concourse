package migration

//go:generate counterfeiter . Bindata

type Bindata interface {
	AssetNames() []string
	Asset(name string) ([]byte, error)
}

type bindataSource struct{}

func (bs *bindataSource) AssetNames() []string {
	return AssetNames()
}

func (bs *bindataSource) Asset(name string) ([]byte, error) {
	return Asset(name)
}
