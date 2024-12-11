package stocklib

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type twelveDataClient struct {
	c      http.Client
	apiKey string
}

type Quote struct {
	Symbol        string  `json:"symbol,omitempty"`
	Name          string  `json:"name,omitempty"`
	Close         float32 `json:"close,omitempty"`
	PercentChange float32 `json:"percent_change"`
}

type Candles struct {
	
}

const (
	twelveDataBaseURL                   = "https://api.twelvedata.com"
	twelveDataQuoteURIFormatString      = "/quote?symbol=%s"
	twelveDataTimeSeriesURIFormatString = "/time_series?symbol=%s&inverval=15min"
	brokerbotUserAgent                  = "brokerbot"
)

func (c *twelveDataClient) get(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err

	}

	return c.do(req)

}

func (c *twelveDataClient) do(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", "apikey "+c.apiKey)
	req.Header.Add("User-Agent", brokerbotUserAgent)
	return c.c.Do(req)

}

func (c *twelveDataClient) GetQuote(ticker string) (*Quote, error) {
	url := twelveDataBaseURL + fmt.Sprintf(twelveDataQuoteURIFormatString, ticker)
	res, err := c.get(url)
	if err != nil {
		log.Printf("failed to execute request for stock quote: %v", err)
		return nil, err
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		log.Printf("failed to read stock quote response: %v", readErr)
		return nil, readErr
	}

	var tmp Quote
	if unmarshalErr := json.Unmarshal(body, &tmp); unmarshalErr != nil {
		return nil, unmarshalErr
	}
	return &tmp, nil
}

func (c *twelveDataClient) GetCandles(ticker string) (*Quote, error) {
	url := twelveDataBaseURL + fmt.Sprintf(twelveDataTimeSeriesURIFormatString, ticker)
	res, err := c.get(url)
	if err != nil {
		log.Printf("failed to execute request for stock candles: %v", err)
		return nil, err
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		log.Printf("failed to read stock candles response: %v", readErr)
		return nil, readErr
	}

	var tmp Quote
	if unmarshalErr := json.Unmarshal(body, &tmp); unmarshalErr != nil {
		return nil, unmarshalErr
	}
	return &tmp, nil
}
