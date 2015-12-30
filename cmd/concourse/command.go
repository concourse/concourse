package main

type ConcourseCommand struct {
	Web    WebCommand    `command:"web"    description:"Run the web UI and build scheduler."`
	Worker WorkerCommand `command:"worker" description:"Run and register a worker."`
}
