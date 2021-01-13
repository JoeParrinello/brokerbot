package shutdownlib

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	gracefulShutdownSignals = []os.Signal{
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	}

	shutdownHooks []func() error
)

func init() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, gracefulShutdownSignals...)
	go shutdownHandler(sigChan)
}

// AddShutdownHandler registers a handler to be run before shutdown.
func AddShutdownHandler(handler func() error) {
	shutdownHooks = append(shutdownHooks, handler)
}

func shutdownHandler(sigChan chan os.Signal) {
	sig := <-sigChan

	fmt.Println() // Spacer to account for ^C in terminal output.
	log.Printf("Caught signal %q, running %d shutdown handlers.", sig, len(shutdownHooks))

	var wg sync.WaitGroup
	for _, hook := range shutdownHooks {
		wg.Add(1)
		go func(hook func() error) {
			defer wg.Done()
			if err := hook(); err != nil {
				log.Printf("Shutdown hook failed: %v", err)
			}
		}(hook)
	}
	wg.Wait()

	log.Printf("Graceful shutdown complete, exiting.")
	os.Exit(0)
}
