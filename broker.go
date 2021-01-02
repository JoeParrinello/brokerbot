package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/Finnhub-Stock-API/finnhub-go"
	"github.com/bwmarrin/discordgo"
	"github.com/zokypesch/proto-lib/utils"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

var (
	discordToken string
	finnhubToken string

	ctx context.Context

	finnhubClient *finnhub.DefaultApiService
	discordClient *discordgo.Session

	messagePrefix string
	test          bool
)

func init() {
	flag.StringVar(&discordToken, "t", "", "Discord Token")
	flag.StringVar(&finnhubToken, "finnhub", "", "Finnhub Token")
	flag.BoolVar(&test, "test", false, "Run in test mode")
	flag.Parse()
}

func main() {
	log.Printf("DiscordBot starting up")
	initTokens()

	if test {
		messagePrefix = utils.RandStringBytesMaskImprSrcUnsafe(6)
		log.Printf("test mode activated. message prefix: %s", messagePrefix)
	}

	ctx = context.WithValue(context.Background(), finnhub.ContextAPIKey, finnhub.APIKey{
		Key: finnhubToken,
	})

	finnhubClient = finnhub.NewAPIClient(finnhub.NewConfiguration()).DefaultApi

	var err error
	discordClient, err = discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalf("failed to create Discord client: %v", err)
	}

	discordClient.AddHandler(handleMessage)
	discordClient.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages)

	// Open a websocket connection to Discord and begin listening.
	if err = discordClient.Open(); err != nil {
		log.Fatalf("failed to open Discord client: %v", err)
	}

	http.HandleFunc("/", handleDefaultPort)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("defaulting to port %s", port)
	}

	log.Printf("DiscordBot ready to serve on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Printf("DiscordBot shutting down")
		discordClient.Close()
		log.Fatal(err)
	}
}

func initTokens() {
	if discordToken != "" && finnhubToken != "" {
		log.Printf("API tokens have been passed via command-line flags.")
		return
	}

	log.Printf("API tokens have not been passed via command-line flags, checking ENV.")

	var ok bool
	ok, finnhubToken, discordToken = getSecrets()
	if !ok {
		log.Fatalf("API tokens not found in ENV, aborting...")
	}
}

func handleDefaultPort(w http.ResponseWriter, r *http.Request) {
	log.Println("Heartbeat")
	fmt.Fprintln(w, "Hello World!")
}

func handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	/* Validation */
	if m.Author.ID == s.State.User.ID {
		// Prevent the bot from talking to itself.
		return
	}

	msg := strings.TrimSpace(m.Content)

	if !strings.HasPrefix(msg, "!stonks ") {
		return
	}

	ticker := strings.TrimPrefix(msg, "!stonks ")

	if ticker == "" {
		// TODO: Send a help message to the user.
		log.Println("Empty stock ticker")
		return
	}

	/* Serving */

	value, err := getQuoteForTicker(ticker)
	if err != nil {
		msg := fmt.Sprintf("failed to get quote for ticker %q :(", ticker)
		sendMessage(s, m.ChannelID, msg)
		if err != nil {
			log.Printf("failed to send message %q to discord: %v", msg, err)
		}
		log.Fatal(fmt.Sprintf("%s: %v", msg, err))
		return
	}

	// Finnhub returns an empty quote for non-existant tickers.
	if value == 0.0 {
		// TODO: Assume it is a crypto symbol at this point?
		msg := fmt.Sprintf("No Such Ticker: %s", ticker)
		sendMessage(s, m.ChannelID, msg)
		if err != nil {
			log.Printf("failed to send message %q to discord: %v", msg, err)
		}
	}

	msg = fmt.Sprintf("Latest quote for %s: $%.2f", ticker, value)
	log.Println(msg)
	_, err = sendMessage(s, m.ChannelID, msg)
	if err != nil {
		log.Printf("failed to send message %q to discord: %v", msg, err)
	}
}

func getQuoteForTicker(ticker string) (float32, error) {
	quote, _, err := finnhubClient.Quote(ctx, ticker)
	if err != nil {
		return 0, err
	}
	return quote.C, nil
}

func getSecrets() (bool, string, string) {
	success, finnhubKeyPath, discordKeyPath := getTokenPaths()
	if !success {
		log.Println("Failed getting the keypaths")
		return false, "", ""
	}
	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		log.Println("Failed creating secret manager client,", err)
		return false, "", ""
	}

	// Build the requests.
	finnhubRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: finnhubKeyPath,
	}
	discordRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: discordKeyPath,
	}

	// Call the API.
	finnhubResult, err := client.AccessSecretVersion(ctx, finnhubRequest)
	if err != nil {
		log.Println("Failed Getting the Finnhub Key", err)
		return false, "", ""
	}
	discordResult, err := client.AccessSecretVersion(ctx, discordRequest)
	if err != nil {
		log.Println("Failed Getting the Discord Key:", err)
		return false, "", ""
	}

	log.Println("Got the keys from secret manager")
	return true, string(finnhubResult.GetPayload().GetData()), string(discordResult.GetPayload().GetData())
}

func getTokenPaths() (bool, string, string) {
	log.Println("Fetching key paths from env files")
	finnhubKeyPath, finnhubPresent := os.LookupEnv("FINNHUB_KEY_PATH")
	discordKeyPath, discordPresent := os.LookupEnv("DISCORD_KEY_PATH")
	return finnhubPresent && discordPresent, finnhubKeyPath, discordKeyPath
}

func sendMessage(s *discordgo.Session, channelID string, msg string) (*discordgo.Message, error) {
	if test {
		msg = fmt.Sprintf("TEST(%s): %s", messagePrefix, msg)
	}
	return s.ChannelMessageSend(channelID, msg)
}
