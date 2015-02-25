package worker

type ContainerSpec interface{}

type ResourceTypeContainerSpec struct {
	Type string
}

type ImageContainerSpec struct {
	Platform string
	Tags     []string

	Image      string
	Privileged bool
}
