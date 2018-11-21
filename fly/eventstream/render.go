package eventstream

import (
	"fmt"
	"io"
	"strings"

	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/go-concourse/concourse/eventstream"
	"github.com/fatih/color"
)

type RenderOptions struct {
	ShowTimestamp bool
}

func Render(dst io.Writer, src eventstream.EventStream, options RenderOptions) int {
	exitStatus := 0

	for {
		ev, err := src.NextEvent()
		if err != nil {
			if err == io.EOF {
				return exitStatus
			} else {
				eventLog := NewEventLogFromError("%s", err)
				fmt.Fprint(dst, AdditionalFormatting(eventLog, options))
				return 255
			}
		}

		var eventLog EventLog
		switch e := ev.(type) {
		case event.Log:
			eventLog = NewEventLog("%s", e.Payload, e.Time)

		case event.LogV50:
			eventLog = NewEventLog("%s", e.Payload, 0)

		case event.InitializeTask:
			eventLog = NewEventLog("\x1b[1m%s\x1b[0m\n", "initializing", e.Time)

		case event.StartTask:
			buildConfig := e.TaskConfig

			argv := strings.Join(append([]string{buildConfig.Run.Path}, buildConfig.Run.Args...), " ")
			eventLog = NewEventLog("\x1b[1mrunning %s\x1b[0m\n", argv, e.Time)

		case event.FinishTask:
			exitStatus = e.ExitStatus

		case event.Error:
			errCol := ui.ErroredColor.SprintFunc()
			eventLog = NewEventLog("%s\n", errCol(e.Message), 0)

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
				eventLog = NewEventLogFromStatus("unknown status: %s",  e.Status, e.Time)
				fmt.Fprint(dst, AdditionalFormatting(eventLog, options))
				return 255
			}

			printColorFunc := printColor.SprintFunc()
			eventLog = NewEventLog("%s\n", printColorFunc(e.Status), e.Time)
			fmt.Fprint(dst, AdditionalFormatting(eventLog, options))

			return exitStatus
		}
		fmt.Fprint(dst, AdditionalFormatting(eventLog, options))
	}
}
