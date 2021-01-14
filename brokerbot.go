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

	"github.com/Finnhub-Stock-API/finnhub-go"
	"github.com/JoeParrinello/brokerbot/cryptolib"
	"github.com/JoeParrinello/brokerbot/messagelib"
	"github.com/JoeParrinello/brokerbot/secretlib"
	"github.com/JoeParrinello/brokerbot/shutdownlib"
	"github.com/JoeParrinello/brokerbot/stocklib"
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

	shutdownlib.AddShutdownHandler(func() error {
		log.Printf("DiscordBot shutting down connection to Discord.")
		return discordClient.Close()
	})

	http.HandleFunc("/", handleDefaultPort)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("DiscordBot ready to serve on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		discordClient.Close()
		log.Fatal(err)
	}
}

func initTokens() {
	if *discordToken != "" && *finnhubToken != "" {
		return
	}

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

	isBotMention, msg := hasBotMention(s, m)

	if !isBotMention {
		// Ignore bot mentions.
		return
	}

	msg = strings.TrimSpace(msg)
	userInput := strings.TrimPrefix(msg, "!stonks ")
	userInput = strings.ToUpper(userInput)
	expandedString := messagelib.ExpandAliases(userInput)
	tickers := messagelib.DedupeTickerStrings(strings.Split(expandedString, " "))

	cryptoContext, cancelFunc := context.WithCancel(ctx)
	priceFeeds, err := cryptolib.GetPriceFeeds()
	if err != nil {
		log.Printf("Failed to fetch Price Feeds: %s", err);
	} else {
		cryptoContext = context.WithValue(cryptoContext, cryptolib.PriceFeeds, priceFeeds)
	}

	if len(tickers) == 1 && tickers[0] == "" {
		// TODO: Send a help message to the user.
		log.Println("No stock tickers provided")
		return
	} else if len(tickers) == 1 && tickers[0] != "" {
		log.Printf("Processing request for: %s", tickers[0])

		tickerType, ticker := getTickerWithType(tickers[0])

		switch tickerType {
		case stock:
			stocklib.HandleStockTicker(ctx, finnhubClient, s, m, ticker)
		case crypto:
			cryptolib.HandleCryptoTicker(cryptoContext, s, m, ticker)
		}
		return
	} else {
		var tickerValues []*messagelib.TickerValue
		for _, ticker := range tickers {
			var tickerType tickerType
			tickerType, ticker = getTickerWithType(ticker)

			switch tickerType {
			case stock:
				tickerValue, err := stocklib.GetQuoteForStockTicker(ctx, finnhubClient, ticker)
				if err == nil && tickerValue != nil {
					tickerValues = append(tickerValues, tickerValue)
				}
			case crypto:
				tickerValue, err := cryptolib.GetQuoteForCryptoAsset(cryptoContext, ticker)
				if err == nil && tickerValue != nil {
					tickerValues = append(tickerValues, tickerValue)
				}
			}
		}
		if tickerValues != nil && len(tickerValues) > 0 {
			messagelib.SendMessageEmbed(s, m.ChannelID, messagelib.CreateMultiMessageEmbed(tickerValues))
		}
	}
	cancelFunc()
}

func hasBotMention(s *discordgo.Session, m *discordgo.MessageCreate) (bool, string) {
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			return true, strings.NewReplacer(
				"<@"+s.State.User.ID+"> ", "",
				"<@!"+s.State.User.ID+"> ", "",
			).Replace(m.Content)
		}
	}
	return false, ""
}

func getTickerWithType(s string) (tickerType, string) {
	if strings.HasPrefix(s, "$") {
		return crypto, strings.TrimPrefix(s, "$")
	}
	return stock, s
}
