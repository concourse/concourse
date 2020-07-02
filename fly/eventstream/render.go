package eventstream

import (
	"fmt"
	"io"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/fatih/color"
)

type RenderOptions struct {
	ShowTimestamp            bool
	IgnoreEventParsingErrors bool
}

type buildEventsVisitorFunc func(atc.Event) error

func (f buildEventsVisitorFunc) VisitEvent(ev atc.Event) error {
	return f(ev)
}

func Render(dst io.Writer, events concourse.BuildEvents, options RenderOptions) int {
	dstImpl := NewTimestampedWriter(dst, options.ShowTimestamp)

	exitStatus := 0

	visitor := buildEventsVisitorFunc(func(ev atc.Event) error {
		switch e := ev.(type) {
		case event.Log:
			dstImpl.SetTimestamp(e.Time)
			fmt.Fprintf(dstImpl, "%s", e.Payload)

		case event.SelectedWorker:
			dstImpl.SetTimestamp(e.Time)
			fmt.Fprintf(dstImpl, "\x1b[1mselected worker:\x1b[0m %s\n", e.WorkerName)

		case event.InitializeTask:
			dstImpl.SetTimestamp(e.Time)
			fmt.Fprintf(dstImpl, "\x1b[1minitializing\x1b[0m\n")

		case event.StartTask:
			buildConfig := e.TaskConfig

			argv := strings.Join(append([]string{buildConfig.Run.Path}, buildConfig.Run.Args...), " ")
			dstImpl.SetTimestamp(e.Time)
			fmt.Fprintf(dstImpl, "\x1b[1mrunning %s\x1b[0m\n", argv)

		case event.FinishTask:
			exitStatus = e.ExitStatus

		case event.Error:
			errCol := ui.ErroredColor.SprintFunc()
			dstImpl.SetTimestamp(0)
			fmt.Fprintf(dstImpl, "%s\n", errCol(e.Message))

		case event.Status:
			dstImpl.SetTimestamp(e.Time)
			var printColor *color.Color

			switch e.Status {
			case "started":
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
				return fmt.Errorf("unknown status: %s", e.Status)
			}

			printColorFunc := printColor.SprintFunc()
			fmt.Fprintf(dstImpl, "%s\n", printColorFunc(e.Status))
		}
		return nil
	})

	for {
		err := events.Accept(visitor)
		if err != nil {
			if err == io.EOF {
				return exitStatus
			} else if options.IgnoreEventParsingErrors && isEventParseError(err) {
				continue
			} else {
				dstImpl.SetTimestamp(0)
				fmt.Fprintf(dstImpl, "failed to parse next event: %s\n", err)
				return 255
			}
		}
	}
}

func isEventParseError(err error) bool {
	if _, ok := err.(event.UnknownEventTypeError); ok {
		return true
	} else if _, ok := err.(event.UnknownEventVersionError); ok {
		return true
	}
	return false
}
