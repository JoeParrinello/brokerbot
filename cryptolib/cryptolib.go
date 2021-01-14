package cryptolib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/JoeParrinello/brokerbot/messagelib"
	"github.com/bwmarrin/discordgo"
)

const (
	geminiBaseURL      = "https://api.gemini.com"
	geminiPriceFeedURI = "/v1/pricefeed"
)

var (
	// PriceFeeds takes a request scope quotes for the Gemini Crypto Exchange.
	PriceFeeds = contextKey("priceFeeds")
)

type contextKey string

// PriceFeed is a current Gemini provided ticker value.
type PriceFeed struct {
	Pair   string `json:"pair"`
	Price  string `json:"price"`
	Change string `json:"percentChange24h"`
}

// HandleCryptoTicker gets a crypto quote from Finnhub and return an embed to be sent to the user.
func HandleCryptoTicker(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate, ticker string) {
	tickerValue, err := GetQuoteForCryptoAsset(ctx, ticker)
	if err != nil {
		msg := fmt.Sprintf("failed to get quote for asset %q :(", ticker)
		log.Printf(fmt.Sprintf("%s: %v", msg, err))
		messagelib.SendMessage(s, m.ChannelID, msg)
		return
	}

	// Empty quotes are non-existant tickers.
	if tickerValue.Value == 0.0 {
		msg := fmt.Sprintf("No Such Asset: %s", ticker)
		log.Printf(msg)
		messagelib.SendMessage(s, m.ChannelID, msg)
		return
	}

	msgEmbed := messagelib.CreateMessageEmbed(tickerValue)
	log.Printf("%+v", msgEmbed)
	messagelib.SendMessageEmbed(s, m.ChannelID, msgEmbed)
}

// GetQuoteForCryptoAsset returns the TickerValue for Crypto Ticker.
func GetQuoteForCryptoAsset(ctx context.Context, asset string) (*messagelib.TickerValue, error) {
	formattedAsset := strings.ToUpper(asset) + "USD"
	priceFeeds, ok := ctx.Value(PriceFeeds).([]PriceFeed)
	if !ok {
		return nil, errors.New("couldn't unmarshal priceFeeds from context")
	}
	priceFeed, ok := getPriceFeed(priceFeeds, formattedAsset)
	if !ok {
		return &messagelib.TickerValue{Ticker: asset, Value: 0.0, Change: 0.0}, nil
	}

	price, err := strconv.ParseFloat(priceFeed.Price, 32)
	if err != nil {
		return &messagelib.TickerValue{Ticker: asset, Value: 0.0, Change: 0.0}, nil
	}
	change, err := strconv.ParseFloat(priceFeed.Change, 32)
	if err != nil {
		return &messagelib.TickerValue{Ticker: asset, Value: float32(price), Change: 0.0}, nil
	}
	return &messagelib.TickerValue{Ticker: asset, Value: float32(price), Change: float32(change) * 100.0}, nil
}

func getPriceFeed(priceFeeds []PriceFeed, asset string) (*PriceFeed, bool) {
	for i := range priceFeeds {
		if priceFeeds[i].Pair == asset {
			return &priceFeeds[i], true
		}
	}
	return nil, false
}

// GetPriceFeeds returns the current Price points of cryptos traded on Gemini.
func GetPriceFeeds() ([]PriceFeed, error) {
	var priceFeeds []PriceFeed
	url := geminiBaseURL + geminiPriceFeedURI
	geminiClient := http.Client{
		Timeout: time.Second * 2, // Timeout after 2 seconds
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return priceFeeds, err
	}

	req.Header.Set("User-Agent", "brokerbot")

	res, getErr := geminiClient.Do(req)
	if getErr != nil {
		return priceFeeds, getErr
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return priceFeeds, readErr
	}

	json.Unmarshal(body, &priceFeeds)

	return priceFeeds, nil
}
