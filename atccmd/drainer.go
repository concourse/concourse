package atccmd

import "os"

type drainer chan<- struct{}

func (d drainer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	<-signals

	close(d)

	return nil
}
