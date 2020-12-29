package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var (
	Token string
)

func init() {
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.Parse()
}

func main() {
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session:", err)
		return
	}

	dg.AddHandler(respond)

	// Wait here until terminal signal is received.
	fmt.Println("Bot is now running...")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc

	// Cleanly close the Discord session.
	dg.Close()
}

func respond(s *discordgo.Session, m *discordgo.MessageCreate) {
	fmt.Println("Hello World!")
}
