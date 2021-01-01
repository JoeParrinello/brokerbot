package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/Finnhub-Stock-API/finnhub-go"
	"github.com/bwmarrin/discordgo"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
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
	if Token == "" || FinnhubToken == "" {
		log.Println("Token or FinnhubToken undefined from command line.")
		success, finnhubToken, discordToken := getSecrets()
		if !success {
			log.Fatalln("Getting tokens from the ENV failed, and flags not set.")
		}
		FinnhubToken, Token = finnhubToken, discordToken
	}

	finnhubClient = finnhub.NewAPIClient(finnhub.NewConfiguration()).DefaultApi
	auth = context.WithValue(context.Background(), finnhub.ContextAPIKey, finnhub.APIKey{
		Key: FinnhubToken,
	})

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Fatalln("error creating Discord session:", err)
	}

	dg.AddHandler(handleMessage)

	// We only care about receiving message events.
	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages)

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Fatalln("error opening discord connection,", err)
	}

	http.HandleFunc("/", handleDefaultPort)
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("defaulting to port %s", port)
	}

	log.Printf("listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Println("Bot is closing...")
		// Cleanly close the listening server.
		l.Close()
		// Cleanly close the Discord session.
		dg.Close()
		log.Println("Bot is closed")
		log.Fatal(err)
        }

	// Wait here until terminal signal is received.
	log.Println("Bot is now running...")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc

	log.Println("Bot is closing...")
	// Cleanly close the listening server.
	l.Close()
	// Cleanly close the Discord session.
	dg.Close()
	log.Println("Bot is closed")
}

func handleDefaultPort(w http.ResponseWriter, r *http.Request) {
	log.Println("Heartbeat")
	fmt.Fprintln(w, "Hello World!")
}

func handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		log.Println("Ignore the bot's message itself")
		return
	}

	trimmed := strings.TrimSpace(m.Content)

	// If not in the format "!stonks <ticker>", give up.
	if !strings.HasPrefix(trimmed, "!stonks ") {
		log.Println("Ignoring non-prefixed message")
		return
	}

	ticker := strings.TrimPrefix(m.Content, "!stonks ")

	if ticker == "" {
		log.Println("Empty stock ticker")
		return
	}

	value, err := findTicker(ticker)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error with Ticker: %s", ticker))
		log.Fatalln("error fetching ticker:", ticker, err)
		return
	}
	output := fmt.Sprintf("Last Ticker Price for %s: %f", ticker, value)
	log.Println(output)
	_, err = s.ChannelMessageSend(m.ChannelID, output)
	if err != nil {
		log.Println("error sending message to discord", err)
	}
}

func findTicker(ticker string) (float32, error) {
	quote, _, err := finnhubClient.Quote(auth, ticker)

	if err != nil {
		log.Println("Error getting quote")
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
