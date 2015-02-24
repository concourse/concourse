package worker

type ContainerSpec interface{}

type ResourceTypeContainerSpec struct {
	Type string
}

type ImageContainerSpec struct {
	Image      string
	Privileged bool
}
