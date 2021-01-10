package shutdownlib

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var gracefulShutdownSignals = []os.Signal{
	syscall.SIGHUP,
	syscall.SIGINT,
	syscall.SIGTERM,
	syscall.SIGQUIT,
}

// AddShutdownHooks registers a shutdown handler to be run upon receiving various OS signals.
func AddShutdownHooks(s *discordgo.Session) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, gracefulShutdownSignals...)
	go shutdownHandler(sigChan, s)
}

func shutdownHandler(sigChan chan os.Signal, s *discordgo.Session) {
	sig := <-sigChan
	fmt.Println() // Spacer to account for ^C in terminal output.
	log.Printf("DiscordBot caught signal %q, shutting down connection to Discord.", sig)
	s.Close()
	log.Printf("DiscordBot shutting down gracefully.")
	os.Exit(0)
}
