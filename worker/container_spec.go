package worker

import (
	"fmt"
	"strings"
)

type ContainerSpec interface {
	Description() string
}

type ResourceTypeContainerSpec struct {
	Type      string
	Ephemeral bool
}

func (spec ResourceTypeContainerSpec) Description() string {
	return fmt.Sprintf("resource type '%s'", spec.Type)
}

type ExecuteContainerSpec struct {
	Platform string
	Tags     []string

	Image      string
	Privileged bool
}

func (spec ExecuteContainerSpec) Description() string {
	messages := []string{
		fmt.Sprintf("platform '%s'", spec.Platform),
	}

	for _, tag := range spec.Tags {
		messages = append(messages, fmt.Sprintf("tag '%s'", tag))
	}

	return strings.Join(messages, ", ")
}
