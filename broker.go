package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/Finnhub-Stock-API/finnhub-go"
	"github.com/bwmarrin/discordgo"
	"github.com/zokypesch/proto-lib/utils"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

var (
	discordToken   string
	finnhubToken   string
	cryptoExchange string

	ctx context.Context

	finnhubClient *finnhub.DefaultApiService
	discordClient *discordgo.Session

	messagePrefix string
	test          bool
)

func init() {
	flag.StringVar(&discordToken, "t", "", "Discord Token")
	flag.StringVar(&finnhubToken, "finnhub", "", "Finnhub Token")
	flag.StringVar(&cryptoExchange, "exchange", "GEMINI", "Crypto Exchange")
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
	ticker = strings.ToUpper(ticker)

	if ticker == "" {
		// TODO: Send a help message to the user.
		log.Println("Empty stock ticker")
		return
	}

	/* Serving */
	log.Printf("Processing request for: %s", ticker)

	value, err := getQuoteForStockTicker(ticker)
	if err != nil {
		msg := fmt.Sprintf("failed to get quote for ticker %q :(", ticker)
		log.Printf(fmt.Sprintf("%s: %v", msg, err))
		sendMessage(s, m.ChannelID, msg)
		return
	}

	// TODO(MatthewLavine): Query for stock and cryto quotes concurrently to speed things up.

	// Finnhub returns an empty quote for non-existant tickers.
	if value == 0.0 {
		var err error
		value, err = getQuoteForCryptoAsset(ticker)
		if err != nil {
			msg := fmt.Sprintf("failed to get quote for asset %q :(", ticker)
			log.Printf(fmt.Sprintf("%s: %v", msg, err))
			sendMessage(s, m.ChannelID, msg)
			return
		}

		if value == 0.0 {
			msg := fmt.Sprintf("No Such Asset: %s", ticker)
			log.Printf(msg)
			sendMessage(s, m.ChannelID, msg)
			return
		}
	}

	msgEmbed := createMessageEmbed(ticker, value)
	log.Println(msgEmbed)
	sendMessageEmbed(s, m.ChannelID, msgEmbed)
}

func getQuoteForStockTicker(ticker string) (float32, error) {
	quote, _, err := finnhubClient.Quote(ctx, ticker)
	if err != nil {
		return 0, err
	}
	return quote.C, nil
}

func getQuoteForCryptoAsset(asset string) (float32, error) {
	// Finnhub takes symbols in the format "GEMINI:btcusd"
	formattedAsset := cryptoExchange + ":" + strings.ToLower(asset) + "usd"
	quote, _, err := finnhubClient.CryptoCandles(ctx,
		/* symbol= */ formattedAsset,
		/* resolution= */ "1", // 1 = 1 hour
		/* from= */ time.Now().Add(-1*time.Minute).Unix(),
		/* to= */ time.Now().Unix())
	if err != nil {
		return 0, err
	}
	if len(quote.C) == 0 {
		return 0, nil
	}
	return quote.C[0], nil
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

func sendMessage(s *discordgo.Session, channelID string, msg string) *discordgo.Message {
	msg = fmt.Sprintf("%s%s", getMessagePrefix(), msg)
	message, err := s.ChannelMessageSend(channelID, msg)
	if err != nil {
		log.Printf("failed to send message %q to discord: %v", msg, err)
	}
	return message
}

func sendMessageEmbed(s *discordgo.Session, channelID string, msg *discordgo.MessageEmbed) *discordgo.Message {
	message, err := s.ChannelMessageSendEmbed(channelID, msg)
	if err != nil {
		log.Printf("failed to send message %+v to discord: %v", msg, err)
	}
	return message
}

func createMessageEmbed(ticker string, value float32) *discordgo.MessageEmbed {
	return createMessageEmbedWithPrefix(ticker, value, getMessagePrefix())
}

func createMessageEmbedWithPrefix(ticker string, value float32, prefix string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       ticker,
		Description: fmt.Sprintf("%sLatest quote for %s: $%.2f", prefix, ticker, value),
	}
}

func getMessagePrefix() string {
	if test {
		return fmt.Sprintf("TEST(%s): ", messagePrefix)
	}
	return ""
}
