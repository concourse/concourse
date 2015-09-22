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
	Tags      []string
	Env       []string
}

func (spec ResourceTypeContainerSpec) Description() string {
	return fmt.Sprintf("resource type '%s'", spec.Type)
}

type TaskContainerSpec struct {
	Platform   string
	Image      string
	Privileged bool
	Tags       []string
}

func (spec TaskContainerSpec) Description() string {
	messages := []string{
		fmt.Sprintf("platform '%s'", spec.Platform),
	}

	for _, tag := range spec.Tags {
		messages = append(messages, fmt.Sprintf("tag '%s'", tag))
	}

	return strings.Join(messages, ", ")
}
