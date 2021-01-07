package messagelib

import (
	"fmt"
	"log"
	"math"

	"github.com/bwmarrin/discordgo"
)

var (
	test          bool   = false
	messagePrefix string = "TEST"
)

// TickerValue passes values of fetched content.
type TickerValue struct {
	Ticker string
	Value  float32
	Change float32
}

// EnterTestModeWithPrefix enables extra log prefixes to identify a test server.
func EnterTestModeWithPrefix(prefix string) {
	test = true
	messagePrefix = prefix
	log.Printf("Test mode activated. Message Prefix: %q", prefix)
}

// ExitTestMode disables extra log prefixes to identify a test server.
func ExitTestMode() {
	test = false
}

// SendMessage sends a plaintext message to a Discord channel.
func SendMessage(s *discordgo.Session, channelID string, msg string) *discordgo.Message {
	msg = fmt.Sprintf("%s: %s", getMessagePrefix(), msg)
	message, err := s.ChannelMessageSend(channelID, msg)
	if err != nil {
		log.Printf("failed to send message %q to discord: %v", msg, err)
	}
	return message
}

// SendMessageEmbed sends a rich "embed" message to a Discord channel.
func SendMessageEmbed(s *discordgo.Session, channelID string, msg *discordgo.MessageEmbed) *discordgo.Message {
	message, err := s.ChannelMessageSendEmbed(channelID, msg)
	if err != nil {
		log.Printf("failed to send message %+v to discord: %v", msg, err)
	}
	return message
}

// CreateMessageEmbed creates a rich Discord "embed" message
func CreateMessageEmbed(tickerValue *TickerValue) *discordgo.MessageEmbed {
	return createMessageEmbedWithPrefix(tickerValue, getMessagePrefix())
}

func createMessageEmbedWithPrefix(tickerValue *TickerValue, prefix string) *discordgo.MessageEmbed {
	if tickerValue == nil {
		return nil
	}

	mesg := fmt.Sprintf("Latest Quote: $%.2f", tickerValue.Value)
	if !math.IsNaN(float64(tickerValue.Change)) && tickerValue.Change != 0 {
		mesg = fmt.Sprintf("%s (%.2f%%)", mesg, tickerValue.Change)
	}
	return &discordgo.MessageEmbed{
		Title:       tickerValue.Ticker,
		URL:         fmt.Sprintf("https://www.google.com/search?q=%s", tickerValue.Ticker),
		Description: mesg,
		Footer: &discordgo.MessageEmbedFooter{
			Text: prefix,
		},
	}
}

// CreateMultiMessageEmbed will return an embedded message for multiple tickers.
func CreateMultiMessageEmbed(tickers []*TickerValue) *discordgo.MessageEmbed {
	return createMultiMessageEmbedWithPrefix(tickers, getMessagePrefix())
}

func createMultiMessageEmbedWithPrefix(tickers []*TickerValue, prefix string) *discordgo.MessageEmbed {
	messageFields := make([]*discordgo.MessageEmbedField, len(tickers))
	for i, ticker := range tickers {
		messageFields[i] = createMessageEmbedField(ticker)
	}
	return &discordgo.MessageEmbed{
		Fields: messageFields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: prefix,
		},
	}
}

func createMessageEmbedField(tickerValue *TickerValue) *discordgo.MessageEmbedField {
	if math.IsNaN(float64(tickerValue.Value)) || tickerValue.Value == 0.0 {
		return &discordgo.MessageEmbedField{
			Name:   tickerValue.Ticker,
			Value:  "No Data",
			Inline: true,
		}
	}

	mesg := fmt.Sprintf("$%.2f", tickerValue.Value)
	if !math.IsNaN(float64(tickerValue.Change)) && tickerValue.Change != 0 {
		mesg = fmt.Sprintf("%s (%.2f%%)", mesg, tickerValue.Change)
	}
	return &discordgo.MessageEmbedField{
		Name:   tickerValue.Ticker,
		Value:  mesg,
		Inline: true,
	}
}

func getMessagePrefix() string {
	if test {
		return messagePrefix
	}
	return ""
}
