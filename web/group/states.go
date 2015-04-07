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

func UnhighlightedStates(config atc.GroupConfigs) []State {
	return States(config, func(atc.GroupConfig) bool {
		return false
	})
}
