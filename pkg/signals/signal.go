package signals

import (
	"os"
	"os/signal"
)

var onlyOneSignalHandler = make(chan struct{})

func SetupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler)

	stop := make(chan struct{})
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, shutdownSignals...)

	go func() {
		<-ch
		close(stop)
		<-ch
		os.Exit(1)
	}()

	return stop
}
