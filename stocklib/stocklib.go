package stocklib

import (
	"context"
	"fmt"

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
