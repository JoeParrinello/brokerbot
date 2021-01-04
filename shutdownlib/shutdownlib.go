package shutdownlib

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

// AddShutdownHooks registers a shutdown handler to be run upon receiving various OS signals.
func AddShutdownHooks(s *discordgo.Session) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go shutdownHandler(sigChan, s)
}

func shutdownHandler(sigChan chan os.Signal, s *discordgo.Session) {
	sig := <-sigChan
	log.Printf("Caught %v, shutting down connection to Discord.", sig)
	s.Close()
	log.Printf("DiscordBot shutting down gracefully.")
	os.Exit(0)
}
