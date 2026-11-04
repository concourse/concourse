// init is the process that sits at pid 1 of every Concourse container on
// platforms without the C-based init (cmd/init/init.c). It does nothing
// until signalled, keeping the container alive so Concourse can exec the
// actual command into it later.
package main

import (
	"os"
	"os/signal"
	"syscall"
)

func main() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
}
