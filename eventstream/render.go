package eventstream

import (
	"fmt"
	"io"
	"strings"

	"github.com/concourse/atc/event"
	"github.com/concourse/fly/ui"
	"github.com/concourse/go-concourse/concourse/eventstream"
	"github.com/fatih/color"
)

func Render(dst io.Writer, src eventstream.EventStream) int {
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

		case event.LogV50:
			fmt.Fprintf(dst, "%s", e.Payload)

		case event.InitializeTask:
			fmt.Fprintf(dst, "\x1b[1minitializing\x1b[0m\n")

		case event.StartTask:
			buildConfig := e.TaskConfig

			argv := strings.Join(append([]string{buildConfig.Run.Path}, buildConfig.Run.Args...), " ")
			fmt.Fprintf(dst, "\x1b[1mrunning %s\x1b[0m\n", argv)

		case event.FinishTask:
			exitStatus = e.ExitStatus

		case event.Error:
			errCol := ui.ErroredColor.SprintFunc()
			fmt.Fprintf(dst, "%s\n", errCol(e.Message))

		case event.Status:
			var printColor *color.Color

			switch e.Status {
			case "started":
				continue
			case "succeeded":
				printColor = ui.SucceededColor
			case "failed":
				printColor = ui.FailedColor

				if exitStatus == 0 {
					exitStatus = 1
				}
			case "errored":
				printColor = ui.ErroredColor

				if exitStatus == 0 {
					exitStatus = 2
				}
			case "aborted":
				printColor = ui.AbortedColor

				if exitStatus == 0 {
					exitStatus = 3
				}
			default:
				fmt.Fprintf(dst, "unknown status: %s", e.Status)
				return 255
			}

			printColorFunc := printColor.SprintFunc()
			fmt.Fprintf(dst, "%s\n", printColorFunc(e.Status))

			return exitStatus
		}
	}

	return 255
}
