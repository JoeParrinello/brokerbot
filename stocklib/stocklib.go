package stocklib

import (
	"context"
	"fmt"
	"log"

	"brokerbot/messagelib"

	"github.com/Finnhub-Stock-API/finnhub-go"
	"github.com/bwmarrin/discordgo"
)

// HandleStockTicker gets a stock quote from Finnhub and return an embed to be sent to the user.
func HandleStockTicker(ctx context.Context, f *finnhub.DefaultApiService, s *discordgo.Session, m *discordgo.MessageCreate, ticker string) {
	value, change, err := getQuoteForStockTicker(ctx, f, ticker)
	if err != nil {
		msg := fmt.Sprintf("failed to get quote for ticker %q :(", ticker)
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
	msgEmbed := messagelib.CreateMessageEmbed(ticker, value, change)
	log.Printf("%+v", msgEmbed)
	messagelib.SendMessageEmbed(s, m.ChannelID, msgEmbed)
}

func getQuoteForStockTicker(ctx context.Context, f *finnhub.DefaultApiService, ticker string) (float32, float32, error) {
	quote, _, err := f.Quote(ctx, ticker)
	if err != nil {
		return 0, 0, err
	}
	dailyChangePercent := ((quote.C - quote.Pc) / quote.Pc) * 100
	return quote.C, dailyChangePercent, nil
}
