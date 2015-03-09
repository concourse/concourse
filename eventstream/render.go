package eventstream

import (
	"fmt"
	"io"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	"github.com/mgutz/ansi"
)

func Render(dst io.Writer, src EventStream) int {
	var buildConfig atc.BuildConfig

	exitStatus := 0

	for {
		ev, err := src.NextEvent()
		if err != nil {
			if err == io.EOF {
				return exitStatus
			} else {
				fmt.Fprintf(dst, "failed to parse next event: %s\n", err)
				return 255
			}
		}

		switch e := ev.(type) {
		case event.Log:
			fmt.Fprintf(dst, "%s", e.Payload)

		case event.InitializeExecute:
			buildConfig = e.BuildConfig

			if buildConfig.Image != "" {
				fmt.Fprintf(dst, "\x1b[1minitializing with %s\x1b[0m\n", buildConfig.Image)
			} else {
				fmt.Fprintf(dst, "\x1b[1minitializing\x1b[0m\n")
			}

		case event.StartExecute:
			argv := strings.Join(append([]string{buildConfig.Run.Path}, buildConfig.Run.Args...), " ")
			fmt.Fprintf(dst, "\x1b[1mrunning %s\x1b[0m\n", argv)

		case event.FinishExecute:
			exitStatus = e.ExitStatus

		case event.Error:
			fmt.Fprintf(dst, "%s\n", ansi.Color(e.Message, "red+b"))

		case event.Status:
			var color string

			switch e.Status {
			case "started":
				continue
			case "succeeded":
				color = "green"
			case "failed":
				color = "red"

				if exitStatus == 0 {
					exitStatus = 1
				}
			case "errored":
				color = "magenta"

				if exitStatus == 0 {
					exitStatus = 2
				}
			case "aborted":
				color = "yellow"

				if exitStatus == 0 {
					exitStatus = 3
				}
			default:
				fmt.Fprintf(dst, "unknown status: %s", e.Status)
				return 255
			}

			fmt.Fprintf(dst, "%s\n", ansi.Color(string(e.Status), color))

			return exitStatus
		}
	}

	return 255
}
