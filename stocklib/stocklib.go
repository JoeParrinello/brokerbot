package stocklib

import (
	"context"

	"github.com/Finnhub-Stock-API/finnhub-go"
	"github.com/JoeParrinello/brokerbot/messagelib"
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
	return &messagelib.TickerValue{
		Ticker: ticker,
		Value:  quote.C,
		Change: dailyChangePercent,
	}, nil
}
