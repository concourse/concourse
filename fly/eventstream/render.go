package eventstream

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/go-concourse/concourse/eventstream"
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
			fmt.Fprintf(dst, "%s  %s", unixTimeAsString(e.Time), e.Payload)

		case event.LogV50:
			fmt.Fprintf(dst, "%s", e.Payload)

		case event.InitializeTask:
			fmt.Fprintf(dst, "%s  \x1b[1minitializing\x1b[0m\n", unixTimeAsString(e.Time))

		case event.StartTask:
			buildConfig := e.TaskConfig

			argv := strings.Join(append([]string{buildConfig.Run.Path}, buildConfig.Run.Args...), " ")
			fmt.Fprintf(dst, "%s  \x1b[1mrunning %s\x1b[0m\n", unixTimeAsString(e.Time), argv)

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
				fmt.Fprintf(dst, "%s  unknown status: %s", unixTimeAsString(e.Time), e.Status)
				return 255
			}

			printColorFunc := printColor.SprintFunc()
			fmt.Fprintf(dst, "%s  %s\n", unixTimeAsString(e.Time), printColorFunc(e.Status))

			return exitStatus
		}
	}
}

func unixTimeAsString(timestamp int64) string {
	const posixTimeLayout string = "15:04:05"
	return time.Unix(timestamp, 0).Format(posixTimeLayout)
}
