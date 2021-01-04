package cryptolib

import (
	"BrokerBot/messagelib"
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Finnhub-Stock-API/finnhub-go"
	"github.com/bwmarrin/discordgo"
)

var (
	cryptoExchange string
)

func init() {
	flag.StringVar(&cryptoExchange, "exchange", "GEMINI", "Crypto Exchange")
}

// HandleCryptoTicker gets a crypto quote from Finnhub and return an embed to be sent to the user.
func HandleCryptoTicker(ctx context.Context, f *finnhub.DefaultApiService, s *discordgo.Session, m *discordgo.MessageCreate, ticker string) {
	value, err := getQuoteForCryptoAsset(ctx, f, ticker)
	if err != nil {
		msg := fmt.Sprintf("failed to get quote for asset %q :(", ticker)
		log.Printf(fmt.Sprintf("%s: %v", msg, err))
		messagelib.SendMessage(s, m.ChannelID, msg)
		return
	}

	// Finnhub returns an empty quote for non-existant tickers.
	if value == 0.0 {
		msg := fmt.Sprintf("No Such Asset: %s", ticker)
		log.Printf(msg)
		messagelib.SendMessage(s, m.ChannelID, msg)
		return
	}

	msgEmbed := messagelib.CreateMessageEmbed(ticker, value, 0.0)
	log.Printf("%+v", msgEmbed)
	messagelib.SendMessageEmbed(s, m.ChannelID, msgEmbed)
}

func getQuoteForCryptoAsset(ctx context.Context, f *finnhub.DefaultApiService, asset string) (float32, error) {
	// Finnhub takes symbols in the format "GEMINI:btcusd"
	formattedAsset := cryptoExchange + ":" + strings.ToLower(asset) + "usd"
	quote, _, err := f.CryptoCandles(ctx,
		/* symbol= */ formattedAsset,
		/* resolution= */ "1", // 1 = 1 hour
		/* from= */ time.Now().Add(-1*time.Minute).Unix(),
		/* to= */ time.Now().Unix())
	if err != nil {
		return 0, err
	}
	if len(quote.C) == 0 {
		return 0, nil
	}
	return quote.C[0], nil
}
