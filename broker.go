package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/JoeParrinello/BrokerBot/cryptolib"
	"github.com/JoeParrinello/BrokerBot/messagelib"
	"github.com/JoeParrinello/BrokerBot/secretlib"
	"github.com/JoeParrinello/BrokerBot/shutdownlib"
	"github.com/JoeParrinello/BrokerBot/stocklib"

	"github.com/Finnhub-Stock-API/finnhub-go"
	"github.com/bwmarrin/discordgo"
	"github.com/zokypesch/proto-lib/utils"
)

var (
	buildVersion string = "dev" // sha1 revision used to build the program
	buildTime    string = "0"   // when the executable was built

	discordToken = flag.String("t", "", "Discord Token")
	finnhubToken = flag.String("finnhub", "", "Finnhub Token")
	testMode     = flag.Bool("test", false, "Run in test mode")

	ctx context.Context

	finnhubClient *finnhub.DefaultApiService

	timeSinceLastHeartbeat time.Time
)

type tickerType int

const (
	crypto tickerType = iota
	stock
)

func main() {
	log.Printf("DiscordBot starting up")
	log.Printf("DiscordBot version: %s", buildVersion)
	log.Printf("DiscordBot build time: %s", buildTime)
	flag.Parse()
	initTokens()

	if *testMode {
		messagelib.EnterTestModeWithPrefix(utils.RandStringBytesMaskImprSrcUnsafe(6))
	}

	ctx = context.WithValue(context.Background(), finnhub.ContextAPIKey, finnhub.APIKey{
		Key: *finnhubToken,
	})

	finnhubClient = finnhub.NewAPIClient(finnhub.NewConfiguration()).DefaultApi

	discordClient, err := discordgo.New("Bot " + *discordToken)
	if err != nil {
		log.Fatalf("failed to create Discord client: %v", err)
	}

	// Extend HTTP client timeouts to compensate for Google Cloud Run CPU container scheduling delay.
	discordClient.Client.Transport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   60 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 60 * time.Second,
	}
	discordClient.Client.Timeout = 1 * time.Minute

	discordClient.AddHandler(handleMessage)
	discordClient.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages)

	// Open a websocket connection to Discord and begin listening.
	if err = discordClient.Open(); err != nil {
		log.Fatalf("failed to open Discord client: %v", err)
	}

	shutdownlib.AddShutdownHooks(discordClient)

	http.HandleFunc("/", handleDefaultPort)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("defaulting to port %s", port)
	}

	log.Printf("DiscordBot ready to serve on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		discordClient.Close()
		log.Fatal(err)
	}
}

func initTokens() {
	if *discordToken != "" && *finnhubToken != "" {
		log.Printf("API tokens have been passed via command-line flags.")
		return
	}

	log.Printf("API tokens have not been passed via command-line flags, checking ENV.")

	var ok bool
	ok, *finnhubToken, *discordToken = secretlib.GetSecrets()
	if !ok {
		log.Fatalf("API tokens not found in ENV, aborting...")
	}
}

func handleDefaultPort(w http.ResponseWriter, r *http.Request) {
	log.Println("Heartbeat")
	timeSinceLastHeartbeat = time.Now()
	fmt.Fprintln(w, "Hello World!")
}

func handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	log.Printf("time since last heartbeat: %s", time.Since(timeSinceLastHeartbeat))
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

	var tickerType tickerType
	tickerType, ticker = getTickerWithType(ticker)

	switch tickerType {
	case stock:
		stocklib.HandleStockTicker(ctx, finnhubClient, s, m, ticker)
	case crypto:
		cryptolib.HandleCryptoTicker(ctx, finnhubClient, s, m, ticker)
	}
}

func getTickerWithType(s string) (tickerType, string) {
	if strings.HasPrefix(s, "$") {
		return crypto, strings.TrimPrefix(s, "$")
	}
	return stock, s
}
