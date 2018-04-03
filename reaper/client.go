package reaper

type Client interface {
	DestroyContainers(handles []string) error
}
