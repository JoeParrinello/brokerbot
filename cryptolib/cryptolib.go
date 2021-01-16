package cryptolib

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/JoeParrinello/brokerbot/messagelib"
)

const (
	geminiBaseURL      = "https://api.gemini.com"
	geminiPriceFeedURI = "/v1/pricefeed"
	brokerbotUserAgent = "brokerbot"
)

type contextKey string

// PriceFeed is a current Gemini provided ticker value.
type PriceFeed struct {
	Pair   string `json:"pair"`
	Price  string `json:"price"`
	Change string `json:"percentChange24h"`
}

// GetQuoteForCryptoAsset returns the TickerValue for Crypto Ticker.
func GetQuoteForCryptoAsset(priceFeeds []*PriceFeed, asset string) (*messagelib.TickerValue, error) {
	formattedAsset := strings.ToUpper(asset) + "USD"
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

func getPriceFeed(priceFeeds []*PriceFeed, asset string) (*PriceFeed, bool) {
	for i := range priceFeeds {
		if priceFeeds[i].Pair == asset {
			return priceFeeds[i], true
		}
	}
	return nil, false
}

// GetPriceFeeds returns the current Price points of cryptos traded on Gemini.
func GetPriceFeeds(geminiClient *http.Client) ([]*PriceFeed, error) {
	var priceFeeds []*PriceFeed

	url := geminiBaseURL + geminiPriceFeedURI
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", brokerbotUserAgent)

	res, getErr := geminiClient.Do(req)
	if getErr != nil {
		return nil, getErr
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return nil, readErr
	}

	unmarshalErr := json.Unmarshal(body, &priceFeeds)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	return priceFeeds, nil
}
