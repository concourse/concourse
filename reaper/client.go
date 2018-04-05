package reaper

//go:generate counterfeiter . Client

type Client interface {
	DestroyContainers(handles []string) error
	Ping() error
}
