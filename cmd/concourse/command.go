package main

import "github.com/concourse/atc/atccmd"

type ConcourseCommand struct {
	Web    atccmd.ATCCommand `command:"web" description:"Run the web UI and build scheduler."`
	Worker WorkerCommand     `command:"worker" description:"Run and register a worker."`
}
