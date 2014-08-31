package eventstream

import (
	"fmt"
	"io"
	"strings"

	"github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/event"
	"github.com/mgutz/ansi"
)

type V10Renderer struct {
}

func (V10Renderer) Render(dst io.Writer, src EventStream) int {
	var buildConfig builds.Config

	for {
		ev, err := src.NextEvent()
		if err != nil {
			fmt.Fprintf(dst, "failed to parse next event: %s\n", err)
			return 255
		}

		switch e := ev.(type) {
		case event.Log:
			fmt.Fprintf(dst, "%s", e.Payload)

		case event.Initialize:
			buildConfig = e.BuildConfig
			fmt.Fprintf(dst, "\x1b[1minitializing with %s\x1b[0m\n", buildConfig.Image)

		case event.Start:
			argv := strings.Join(append([]string{buildConfig.Run.Path}, buildConfig.Run.Args...), " ")
			fmt.Fprintf(dst, "\x1b[1mrunning %s\x1b[0m\n", argv)

		case event.Error:
			fmt.Fprintf(dst, "%s", ansi.Color(e.Message, "red+b"))

		case event.Status:
			var exitCode int
			var color string

			switch e.Status {
			case "started":
				continue
			case "succeeded":
				color = "green"
				exitCode = 0
			case "failed":
				color = "red"
				exitCode = 1
			case "errored":
				color = "magenta"
				exitCode = 2
			default:
				fmt.Fprintf(dst, "unknown status: %s", e.Status)
				return 255
			}

			fmt.Fprintf(dst, "%s\n", ansi.Color(string(e.Status), color))

			return exitCode
		}
	}

	return 255
}
