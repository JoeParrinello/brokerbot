package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
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
	geminiClient  *http.Client
)

type tickerType int

const (
	crypto tickerType = iota
	stock

	botHandle = "@BrokerBot"
	botPrefix = "!stonks"
	helpToken = "help"
)

func main() {
	log.Printf("BrokerBot starting up")
	log.Printf("BrokerBot version: %s", buildVersion)
	log.Printf("BrokerBot build time: %s", buildTime)
	flag.Parse()
	initTokens()

	if *testMode {
		messagelib.EnterTestModeWithPrefix(utils.RandStringBytesMaskImprSrcUnsafe(6))
	}

	ctx = context.WithValue(context.Background(), finnhub.ContextAPIKey, finnhub.APIKey{
		Key: *finnhubToken,
	})

	finnhubClient = finnhub.NewAPIClient(finnhub.NewConfiguration()).DefaultApi

	geminiClient = &http.Client{
		Timeout: time.Second * 30,
	}

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
		log.Printf("BrokerBot shutting down connection to Discord.")
		return discordClient.Close()
	})

	http.HandleFunc("/", handleDefaultPort)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr: ":" + port,
	}
	shutdownlib.AddShutdownHandler((func() error {
		log.Printf("BrokerBot shutting HTTP srever.")
		return server.Shutdown(ctx)
	}))

	log.Printf("BrokerBot ready to serve on port %s", port)
	if err := server.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			return
		}
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
	fmt.Fprintln(w, "OK")
}

func handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		// Ignore messages from self.
		return
	}

	splitMsg := strings.Fields(m.ContentWithMentionsReplaced())

	if splitMsg[0] != botHandle && splitMsg[0] != botPrefix {
		// Message wasn't meant for us.
		return
	}

	if len(splitMsg) < 2 || splitMsg[1] == helpToken {
		// Message didn't have enough parameters.
		messagelib.SendMessage(s, m.ChannelID, getHelpMessage())
		return
	}

	var tickers []string = splitMsg[1:]
	tickers = messagelib.RemoveMentions(tickers)
	tickers = messagelib.CanonicalizeMessage(tickers)
	tickers = messagelib.ExpandAliases(tickers)
	tickers = messagelib.DedupeSlice(tickers)

	startTime := time.Now()
	log.Printf("Received request for tickers: %s", tickers)

	tickerValueChan := make(chan *messagelib.TickerValue, len(tickers))
	var wg sync.WaitGroup
	for _, rawTicker := range tickers {
		wg.Add(1)

		go func(rawTicker string) {
			defer wg.Done()
			ticker, tickerType := getTickerAndType(rawTicker)

			switch tickerType {
			case stock:
				tickerValue, err := stocklib.GetQuoteForStockTicker(ctx, finnhubClient, ticker)
				if err != nil {
					msg := fmt.Sprintf("Failed to get quote for stock ticker: %q (See logs)", ticker)
					log.Printf(fmt.Sprintf("%s: %v", msg, err))
					messagelib.SendMessage(s, m.ChannelID, msg)
					return
				}
				tickerValueChan <- tickerValue
			case crypto:
				tickerValue, err := cryptolib.GetQuoteForCryptoAsset(geminiClient, ticker)
				if err != nil {
					msg := fmt.Sprintf("Failed to get quote for crypto ticker: %q (See logs)", ticker)
					log.Printf(fmt.Sprintf("%s: %v", msg, err))
					messagelib.SendMessage(s, m.ChannelID, msg)
					return
				}
				tickerValueChan <- tickerValue
			}
			return
		}(rawTicker)
	}
	wg.Wait()
	close(tickerValueChan)

	var tv []*messagelib.TickerValue
	for t := range tickerValueChan {
		tv = append(tv, t)
	}

	sort.Strings(tickers)
	sort.SliceStable(tv, func(i, j int) bool {
		r := strings.Compare(tv[i].Ticker, tv[j].Ticker)
		if r < 0 {
			return true
		}
		return false
	})

	messagelib.SendMessageEmbed(s, m.ChannelID, messagelib.CreateMultiMessageEmbed(tv))
	log.Printf("Sent response for tickers in %v: %s", time.Since(startTime), tickers)
}

func getTickerAndType(s string) (string, tickerType) {
	if strings.HasPrefix(s, "$") {
		return strings.TrimPrefix(s, "$"), crypto
	}
	return s, stock
}

func getHelpMessage() string {
	return strings.Join([]string{
		"Acceptable formats are:",
		fmt.Sprintf("%s <ticker> <ticker> ...", botHandle),
		"or",
		fmt.Sprintf("%s <ticker> <ticker> ...", botPrefix),
	}, "\n")
}
