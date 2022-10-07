package stocklib

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Finnhub-Stock-API/finnhub-go"
	"github.com/JoeParrinello/brokerbot/messagelib"
	"github.com/antihax/optional"
)

// GetQuoteForStockTicker returns the TickerValue for the provided ticker
func GetQuoteForStockTicker(ctx context.Context, f *finnhub.DefaultApiService, ticker string) (*messagelib.TickerValue, error) {
	quote, _, err := f.Quote(ctx, ticker)
	if err != nil {
		return nil, err
	}
	if quote.C == 0.0 {
		// A value of 0.0 means that the Ticker is Undefined.
		return &messagelib.TickerValue{Ticker: ticker, Value: 0.0, Change: 0.0}, nil
	}
	dailyChangePercent := ((quote.C - quote.Pc) / quote.Pc) * 100
	company, _, err := f.CompanyProfile2(ctx, &finnhub.CompanyProfile2Opts{
		Symbol: optional.NewString(ticker),
	})
	companyName := company.Name
	if err != nil {
		fmt.Printf("Company lookup failed, ignoring: %v", err)
		companyName = "Error"
	}
	if companyName == "" {
		companyName = "Unknown"
	}
	return &messagelib.TickerValue{
		Ticker: fmt.Sprintf("%s (%s)", ticker, companyName),
		Value:  quote.C,
		Change: dailyChangePercent,
	}, nil
}

func GetCandleGraphForStockAsset(ctx context.Context, f *finnhub.DefaultApiService, cloudRunClient *http.Client, ticker string) (string, error) {
	now := time.Now()
	candles, _, err := f.StockCandles(ctx, ticker, "15", now.Add(time.Hour*-24*7).Unix(), now.Unix(), &finnhub.StockCandlesOpts{})
	if err != nil {
		log.Printf("failed to request stock candle: %v", err)
		return "", err
	}

	marshalledReq, marshalError := json.Marshal(candles)
	if marshalError != nil {
		log.Printf("failed to marshal for stock candles graph: %v", marshalError)
		return "", marshalError
	}

	url := os.Getenv("GET_STOCK_CANDLE_GRAPH_URL")
	if url == "" {
		return "", errors.New("no stock candle graph url")
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(marshalledReq))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	res, cloudRunErr := cloudRunClient.Do(req)
	if cloudRunErr != nil {
		log.Printf("failed to execute request for stock candles graph: %v", cloudRunErr)
		return "", cloudRunErr
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
		log.Printf("failed to read stock candles graph response: %v", readErr)
		return "", readErr
	}

	return string(body), nil
}
