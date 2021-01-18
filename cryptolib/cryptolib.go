package cryptolib

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/JoeParrinello/brokerbot/messagelib"
)

var (
	mu          sync.Mutex
	priceFeeds  []*PriceFeed
	lastUpdated time.Time

	priceFeedAgeLimit = flag.Duration("priceFeedAgeLimit", 5*time.Minute, "The maximum age limit of crypto price feeds before we re-fetch them.")
)

const (
	geminiBaseURL      = "https://api.gemini.com"
	geminiPriceFeedURI = "/v1/pricefeed"
	brokerbotUserAgent = "brokerbot"
)

// PriceFeed is a current Gemini provided ticker value.
type PriceFeed struct {
	Pair   string `json:"pair"`
	Price  string `json:"price"`
	Change string `json:"percentChange24h"`
}

// GetQuoteForCryptoAsset returns the TickerValue for Crypto Ticker.
func GetQuoteForCryptoAsset(geminiClient *http.Client, asset string) (*messagelib.TickerValue, error) {
	formattedAsset := asset + "USD"
	priceFeed, ok := getFeedForAsset(geminiClient, formattedAsset)
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

func getFeedForAsset(geminiClient *http.Client, asset string) (*PriceFeed, bool) {
	fetchPriceFeeds(geminiClient)
	for _, feed := range priceFeeds {
		if feed.Pair == asset {
			return feed, true
		}
	}
	return nil, false
}

func fetchPriceFeeds(geminiClient *http.Client) {
	mu.Lock()
	defer mu.Unlock()

	if time.Since(lastUpdated) <= *priceFeedAgeLimit {
		return
	}

	log.Printf("Crypto price feeds are older than %v, fetching update.", *priceFeedAgeLimit)

	var newPriceFeeds []*PriceFeed

	url := geminiBaseURL + geminiPriceFeedURI
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Printf("failed to create request for crypto price feeds: %v", err)
		return
	}

	req.Header.Set("User-Agent", brokerbotUserAgent)

	res, getErr := geminiClient.Do(req)
	if getErr != nil {
		log.Printf("failed to execute request for crypto price feeds: %v", err)
		return
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Printf("failed to read crypto price feed response: %v", err)
		return
	}

	unmarshalErr := json.Unmarshal(body, &newPriceFeeds)
	if unmarshalErr != nil {
		log.Printf("failed to unmarshal crypto price feed response: %v", err)
		return
	}

	priceFeeds = newPriceFeeds
	lastUpdated = time.Now()
}
