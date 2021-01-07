package cryptolib

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Finnhub-Stock-API/finnhub-go"
	"github.com/JoeParrinello/brokerbot/messagelib"
	"github.com/bwmarrin/discordgo"
)

var (
	cryptoExchange = flag.String("exchange", "GEMINI", "Crypto Exchange")
)

// HandleCryptoTicker gets a crypto quote from Finnhub and return an embed to be sent to the user.
func HandleCryptoTicker(ctx context.Context, f *finnhub.DefaultApiService, s *discordgo.Session, m *discordgo.MessageCreate, ticker string) {
	tickerValue, err := GetQuoteForCryptoAsset(ctx, f, ticker)
	if err != nil {
		msg := fmt.Sprintf("failed to get quote for asset %q :(", ticker)
		log.Printf(fmt.Sprintf("%s: %v", msg, err))
		messagelib.SendMessage(s, m.ChannelID, msg)
		return
	}

	// Finnhub returns an empty quote for non-existant tickers.
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
func GetQuoteForCryptoAsset(ctx context.Context, f *finnhub.DefaultApiService, asset string) (*messagelib.TickerValue, error) {
	// Finnhub takes symbols in the format "GEMINI:btcusd"
	formattedAsset := *cryptoExchange + ":" + strings.ToLower(asset) + "usd"
	quote, _, err := f.CryptoCandles(ctx,
		/* symbol= */ formattedAsset,
		/* resolution= */ "1", // 1 = 1 hour
		/* from= */ time.Now().Add(-1*time.Minute).Unix(),
		/* to= */ time.Now().Unix())
	if err != nil {
		return nil, err
	}
	if len(quote.C) == 0 {
		// A value of 0.0 means that the Ticker is Undefined.
		return &messagelib.TickerValue{Ticker: asset, Value: 0.0, Change: 0.0}, nil
	}
	return &messagelib.TickerValue{Ticker: asset, Value: quote.C[0], Change: 0.0}, nil
}
