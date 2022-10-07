package cryptolib

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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

	cryptoNames = map[string]string{
		"BTC":  "Bitcoin",
		"LTC":  "Litecoin",
		"ETH":  "Ethereum",
		"DOGE": "Dogecoin",
		"XRP":  "Ripple",
		"BCH":  "Bitcoin Cash",
		"USDT": "Tether",
		"ZEC":  "Zcash",
		"LINK": "Chainlink",
		"DOT":  "Polkadot",
		"XMR":  "Monero",
		"LUNA": "Terra",
		"DASH": "Dash",
	}
)

const (
	geminiBaseURL                = "https://api.gemini.com"
	geminiPriceFeedURI           = "/v1/pricefeed"
	geminiCandlesURIFormatString = "/v2/candles/%s/15m"
	brokerbotUserAgent           = "brokerbot"
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
		return &messagelib.TickerValue{Ticker: assetWithName(asset), Value: 0.0, Change: 0.0}, nil
	}

	price, err := strconv.ParseFloat(priceFeed.Price, 32)
	if err != nil {
		return &messagelib.TickerValue{Ticker: assetWithName(asset), Value: 0.0, Change: 0.0}, nil
	}
	change, err := strconv.ParseFloat(priceFeed.Change, 32)
	if err != nil {
		return &messagelib.TickerValue{Ticker: assetWithName(asset), Value: float32(price), Change: 0.0}, nil
	}
	return &messagelib.TickerValue{Ticker: assetWithName(asset), Value: float32(price), Change: float32(change) * 100.0}, nil
}

func GetCandleGraphForCryptoAsset(geminiClient *http.Client, cloudRunClient *http.Client, asset string) (string, error) {
	candlesData, err := FetchCandles(geminiClient, asset)
	if err != nil {
		return "", err
	}

	url := os.Getenv("GET_CRYPTO_CANDLE_GRAPH_URL")
	if url == "" {
		return "", errors.New("no crypto candle graph url")
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(candlesData))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	res, err := cloudRunClient.Do(req)
	if err != nil {
		log.Printf("failed to execute request for crypto candles graph: %v", err)
		return "", err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	if res.StatusCode != 200 {
		log.Print("crypto candle image did not return")
		return "", errors.New("crypto candle image did not return")
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Printf("failed to read crypto candles graph response: %v", readErr)
		return "", readErr
	}

	return string(body), nil
}

func getFeedForAsset(geminiClient *http.Client, asset string) (*PriceFeed, bool) {
	FetchPriceFeeds(geminiClient)
	for _, feed := range priceFeeds {
		if feed.Pair == asset {
			return feed, true
		}
	}
	return nil, false
}

func assetWithName(asset string) string {
	name, ok := cryptoNames[asset]
	if !ok {
		return asset
	}
	return fmt.Sprintf("%s (%s)", asset, name)
}

func GetLatestPriceFeed() []*PriceFeed {
	mu.Lock()
	defer mu.Unlock()
	return priceFeeds
}

func GetLatestPriceFeedUpdateTime() time.Time {
	mu.Lock()
	defer mu.Unlock()
	return lastUpdated
}

func FetchPriceFeeds(geminiClient *http.Client) {
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

func FetchCandles(geminiClient *http.Client, asset string) ([]byte, error) {
	formattedAsset := asset + "USD"
	mu.Lock()
	defer mu.Unlock()

	url := geminiBaseURL + fmt.Sprintf(geminiCandlesURIFormatString, formattedAsset)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Printf("failed to create request for crypto candle: %v", err)
		return nil, err
	}

	req.Header.Set("User-Agent", brokerbotUserAgent)

	res, getErr := geminiClient.Do(req)
	if getErr != nil {
		log.Printf("failed to execute request for crypto candles: %v", err)
		return nil, getErr
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Printf("failed to read crypto candles response: %v", err)
		return nil, readErr
	}

	var tmp []interface{}
	if unmarshalErr := json.Unmarshal(body, &tmp); unmarshalErr != nil {
		return nil, unmarshalErr
	}

	if len(tmp) < 672 {
		return body, nil
	}

	newReturn, marshallErr := json.Marshal(tmp[:672])
	if marshallErr != nil {
		return nil, marshallErr
	}
	return newReturn, nil
}
