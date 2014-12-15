package auth

import "github.com/concourse/turbine/event"

func Censor(e event.Event) event.Event {
	switch ev := e.(type) {
	case event.Initialize:
		ev.BuildConfig.Params = nil
		return ev
	case event.Input:
		ev.Input.Source = nil
		ev.Input.Params = nil
		return ev
	case event.Output:
		ev.Output.Source = nil
		ev.Output.Params = nil
		return ev
	default:
		return ev
	}
}
