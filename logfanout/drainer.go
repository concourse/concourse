package logfanout

import "os"

type Drainer struct {
	Tracker *Tracker
}

func (drainer Drainer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	<-signals

	drainer.Tracker.Drain()

	return nil
}
