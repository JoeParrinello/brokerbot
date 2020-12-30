package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Finnhub-Stock-API/finnhub-go"
	"github.com/bwmarrin/discordgo"
)

var (
	Token         string
	FinnhubToken  string
	finnhubClient *finnhub.DefaultApiService
	auth          context.Context
)

func init() {
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.StringVar(&FinnhubToken, "finnhub", "", "Finnhub Token")
	flag.Parse()
}

func main() {
	finnhubClient = finnhub.NewAPIClient(finnhub.NewConfiguration()).DefaultApi
	auth = context.WithValue(context.Background(), finnhub.ContextAPIKey, finnhub.APIKey{
		Key: FinnhubToken,
	})

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session:", err)
		return
	}

	dg.AddHandler(handleMessage)

	// We only care about receiving message events.
	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages)

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until terminal signal is received.
	fmt.Println("Bot is now running...")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc

	// Cleanly close the Discord session.
	dg.Close()
}

func handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	trimmed := strings.TrimSpace(m.Content)

	// If not in the format "!stonks <ticker>", give up.
	if !strings.HasPrefix(trimmed, "!stonks ") {
		return
	}

	ticker := strings.TrimPrefix(m.Content, "!stonks ")

	if ticker == "" {
		return
	}

	value, err := findTicker(ticker)
	if err != nil {
		fmt.Println("Error fetching ticker,", err)
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error with Ticker: %s", ticker))
		return
	}
	output := fmt.Sprintf("Last Ticker Price for %s: %f", ticker, value)
	fmt.Println(output)
	s.ChannelMessageSend(m.ChannelID, output)
}

func findTicker(ticker string) (float32, error) {
	quote, _, err := finnhubClient.Quote(auth, ticker)

	if err != nil {
		return 0, err
	}

	return quote.C, nil
}
