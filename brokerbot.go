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
	"github.com/JoeParrinello/brokerbot/firestorelib"
	"github.com/JoeParrinello/brokerbot/messagelib"
	"github.com/JoeParrinello/brokerbot/secretlib"
	"github.com/JoeParrinello/brokerbot/shutdownlib"
	"github.com/JoeParrinello/brokerbot/statuszlib"
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

	botPrefixes = []string{"!stonks", "!stnosk", "!stonsk"}
)

type tickerType int

const (
	crypto tickerType = iota
	stock

	aliasToken = "alias"
	botHandle  = "@BrokerBot"
	helpToken  = "help"
)

func main() {
	flag.Parse()
	initTokens()
	log.Printf("BrokerBot starting up")
	log.Printf("BrokerBot version: %s", buildVersion)
	log.Printf("BrokerBot build time: %s", buildTime)

	statuszlib.SetBuildVersion(buildVersion)
	statuszlib.SetBuildTime(buildTime)

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
	cryptolib.FetchPriceFeeds(geminiClient)

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

	firestorelib.Init()

	http.HandleFunc("/", handleDefaultPort)

	http.HandleFunc("/statusz", statuszlib.HandleStatusz)

	// Google Cloud blocks /statusz, so also bind to /status
	http.HandleFunc("/status", statuszlib.HandleStatusz)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	httpServer := &http.Server{
		Addr: ":" + port,
	}
	shutdownlib.AddShutdownHandler((func() error {
		log.Printf("BrokerBot shutting down HTTP server.")
		return httpServer.Shutdown(ctx)
	}))

	log.Printf("BrokerBot ready to serve on port %s", port)
	if err := httpServer.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			discordClient.Close()
			log.Fatal(err)
		}
	}
	shutdownlib.WaitForShutdown()
}

func initTokens() {
	if *discordToken != "" && *finnhubToken != "" {
		return
	}

	log.SetFlags(0) // Disable timestamps when using Cloud Logging.

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

	if len(splitMsg) == 0 {
		// Message had no text and was probably an image.
		return
	}

	if splitMsg[0] != botHandle && !contains(botPrefixes, splitMsg[0]) {
		// Message wasn't meant for us.
		return
	}

	statuszlib.RecordRequest()

	if len(splitMsg) < 2 || splitMsg[1] == helpToken {
		// Message didn't have enough parameters.
		messagelib.SendMessage(s, m.ChannelID, getHelpMessage())
		return
	}

	if splitMsg[1] == aliasToken {
		if len(splitMsg) < 3 {
			// Message didn't have enough parameters.
			messagelib.SendMessage(s, m.ChannelID, getHelpMessage())
			return
		}
		switch splitMsg[2] {
		case "list":
			if len(splitMsg) < 3 {
				// Message didn't have enough parameters.
				messagelib.SendMessage(s, m.ChannelID, getHelpMessage())
				return
			}
			aliases, err := firestorelib.GetAliases(ctx)
			if err != nil {
				msg := fmt.Sprintf("failed to get alias: %v", err)
				log.Println(msg)
				messagelib.SendMessage(s, m.ChannelID, msg)
				statuszlib.RecordError()
				return
			}
			var b strings.Builder
			for alias, assets := range aliases {
				b.WriteString(fmt.Sprintf("%s: %s\n", alias, strings.Join(assets, ", ")))
			}
			messagelib.SendMessage(s, m.ChannelID, b.String())
			statuszlib.RecordSuccess()
			return
		case "get":
			if len(splitMsg) < 4 {
				// Message didn't have enough parameters.
				messagelib.SendMessage(s, m.ChannelID, getHelpMessage())
				return
			}
			alias, err := firestorelib.GetAlias(ctx, splitMsg[3])
			if err != nil {
				msg := fmt.Sprintf("failed to get alias: %v", err)
				log.Println(msg)
				messagelib.SendMessage(s, m.ChannelID, msg)
				statuszlib.RecordError()
				return
			}
			messagelib.SendMessage(s, m.ChannelID, strings.Join(alias, ", "))
			statuszlib.RecordSuccess()
			return
		case "set":
			if len(splitMsg) < 5 || !strings.HasPrefix(splitMsg[3], "?") {
				// Message didn't have enough parameters.
				messagelib.SendMessage(s, m.ChannelID, getHelpMessage())
				return
			}
			if err := firestorelib.CreateAlias(ctx, splitMsg[3], splitMsg[4:]); err != nil {
				msg := fmt.Sprintf("failed to create alias: %v", err)
				log.Println(msg)
				messagelib.SendMessage(s, m.ChannelID, msg)
				statuszlib.RecordError()
				return
			}
			messagelib.SendMessage(s, m.ChannelID, fmt.Sprintf("Created alias %q", splitMsg[3]))
			statuszlib.RecordSuccess()
			return
		case "delete":
			if len(splitMsg) < 4 {
				// Message didn't have enough parameters.
				messagelib.SendMessage(s, m.ChannelID, getHelpMessage())
				return
			}
			if err := firestorelib.DeleteAlias(ctx, splitMsg[3]); err != nil {
				msg := fmt.Sprintf("failed to delete alias: %v", err)
				log.Println(msg)
				messagelib.SendMessage(s, m.ChannelID, msg)
				statuszlib.RecordError()
				return
			}
			messagelib.SendMessage(s, m.ChannelID, fmt.Sprintf("Deleted alias %q", splitMsg[3]))
			statuszlib.RecordSuccess()
			return
		}
		// Message didn't have enough parameters.
		messagelib.SendMessage(s, m.ChannelID, getHelpMessage())
		return
	}

	var tickers []string = splitMsg[1:]
	tickers = messagelib.RemoveMentions(tickers)
	tickers = messagelib.CanonicalizeMessage(tickers)

	var err error
	tickers, err = messagelib.ExpandAliases(ctx, tickers)
	if err != nil {
		msg := fmt.Sprintf("failed to expand aliases: %v", err)
		log.Println(msg)
		messagelib.SendMessage(s, m.ChannelID, msg)
		statuszlib.RecordError()
		return
	}

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
					log.Printf("%s: %v", msg, err)
					messagelib.SendMessage(s, m.ChannelID, msg)
					statuszlib.RecordError()
					return
				}
				tickerValueChan <- tickerValue
			case crypto:
				tickerValue, err := cryptolib.GetQuoteForCryptoAsset(geminiClient, ticker)
				if err != nil {
					msg := fmt.Sprintf("Failed to get quote for crypto ticker: %q (See logs)", ticker)
					log.Printf("%s: %v", msg, err)
					messagelib.SendMessage(s, m.ChannelID, msg)
					statuszlib.RecordError()
					return
				}
				tickerValueChan <- tickerValue
			}
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
		return r < 0
	})

	messagelib.SendMessageEmbed(s, m.ChannelID, messagelib.CreateMultiMessageEmbed(tv))
	log.Printf("Sent response for tickers in %v: %s", time.Since(startTime), tickers)
	statuszlib.RecordSuccess()
}

func getTickerAndType(s string) (string, tickerType) {
	if strings.HasPrefix(s, "$") {
		return strings.TrimPrefix(s, "$"), crypto
	}
	return s, stock
}

func getHelpMessage() string {
	return strings.Join([]string{
		"Invoke bot with either:",
		"  @BrokerBot <ticker> <ticker> ...",
		"  or",
		"  !stonks <ticker> <ticker> ...",
		"",
		"Other commands:",
		"  !stonks help",
		"  !stonks alias list",
		"  !stonks alias get ?<alias>",
		"  !stonks alias set ?<alias> <ticker> <ticker> ...",
		"  !stonks alias delete ?<alias>",
	}, "\n")
}

func contains(s []string, v string) bool {
	for _, a := range s {
		if a == v {
			return true
		}
	}
	return false
}
