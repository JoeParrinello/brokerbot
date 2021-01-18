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

	isShutdown    bool
	shutdownHooks []func() error
	mu            sync.Mutex

	shutdownChan = make(chan struct{})
)

func init() {
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, gracefulShutdownSignals...)
	go shutdownHandler(sigChan)
}

// WaitForShutdown blocks on a channel that will never be closed.
// Its purpose is to be called by main to prevent server exit while
// shutdown handler goroutines are executing.
func WaitForShutdown() {
	<-shutdownChan
}

// AddShutdownHandler registers a handler to be run before shutdown.
func AddShutdownHandler(handler func() error) {
	mu.Lock()
	defer mu.Unlock()
	shutdownHooks = append(shutdownHooks, handler)
}

func shutdownHandler(sigChan chan os.Signal) {
	for sig := range sigChan {
		go func(sig os.Signal) {
			// If we get a second kill signal, exit immediately.
			mu.Lock()
			if isShutdown {
				log.Printf("Received second shutdown signal %q, exiting immediately.", sig)
				os.Exit(1)
			}
			isShutdown = true
			mu.Unlock()

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
		}(sig)
	}
}
