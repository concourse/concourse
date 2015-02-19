package group

import "github.com/concourse/atc"

type State struct {
	Name    string
	Enabled bool
}

func States(config atc.GroupConfigs, pred func(atc.GroupConfig) bool) []State {
	var states []State

	for _, group := range config {
		states = append(states, State{
			Name:    group.Name,
			Enabled: pred(group),
		})
	}

	return states
}
