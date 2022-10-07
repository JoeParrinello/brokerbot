package messagelib

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/JoeParrinello/brokerbot/firestorelib"
	"github.com/bwmarrin/discordgo"
)

var (
	test          bool   = false
	messagePrefix string = "TEST"
)

// TickerValue passes values of fetched content.
type TickerValue struct {
	Ticker   string
	Value    float32
	Change   float32
	ChartUrl string
}

// EnterTestModeWithPrefix enables extra log prefixes to identify a test server.
func EnterTestModeWithPrefix(prefix string) {
	test = true
	messagePrefix = prefix
	log.Printf("BrokerBot running in test mode with prefix: %q", prefix)
}

// ExitTestMode disables extra log prefixes to identify a test server.
func ExitTestMode() {
	test = false
}

// SendMessage sends a plaintext message to a Discord channel.
func SendMessage(s *discordgo.Session, channelID string, msg string) *discordgo.Message {
	msg = fmt.Sprintf("%s%s", getMessagePrefix(), msg)
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
	return createMessageEmbedWithPrefix(tickerValue, getTestServerID())
}

func createMessageEmbedWithPrefix(tickerValue *TickerValue, prefix string) *discordgo.MessageEmbed {
	if tickerValue == nil {
		return nil
	}

	mesg := fmt.Sprintf("Latest Quote: $%s", formatFloat(tickerValue.Value, 4))
	if !math.IsNaN(float64(tickerValue.Change)) && tickerValue.Change != 0 {
		mesg = fmt.Sprintf("%s (%s%%)", mesg, formatFloat(tickerValue.Change, 4))
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
	return createMultiMessageEmbedWithPrefix(tickers, getTestServerID())
}

func createMultiMessageEmbedWithPrefix(tickers []*TickerValue, prefix string) *discordgo.MessageEmbed {
	messageFields := make([]*discordgo.MessageEmbedField, len(tickers))
	for i, ticker := range tickers {
		messageFields[i] = createMessageEmbedField(ticker)
	}
	if len(tickers) == 1 && tickers[0].ChartUrl != "" {
		return &discordgo.MessageEmbed{
			Fields: messageFields,
			Footer: &discordgo.MessageEmbedFooter{
				Text: prefix,
			},
			Image: &discordgo.MessageEmbedImage{
				URL: tickers[0].ChartUrl,
			},
		}
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
			Inline: false,
		}
	}

	mesg := fmt.Sprintf("$%s", formatFloat(tickerValue.Value, 4))
	if !math.IsNaN(float64(tickerValue.Change)) && tickerValue.Change != 0 {
		mesg = fmt.Sprintf("%s (%s%%)", mesg, formatFloat(tickerValue.Change, 4))
	}
	return &discordgo.MessageEmbedField{
		Name:   tickerValue.Ticker,
		Value:  mesg,
		Inline: false,
	}
}

func formatFloat(num float32, prc int) string {
	str := fmt.Sprintf("%."+strconv.Itoa(prc)+"f", num)
	return strings.TrimRight(strings.TrimRight(str, "0"), ".")
}

func getMessagePrefix() string {
	if test {
		return messagePrefix + ": "
	}
	return ""
}

func getTestServerID() string {
	if test {
		return messagePrefix
	}
	return ""
}

// RemoveMentions removes any @ mentions from a message slice.
func RemoveMentions(s []string) (ret []string) {
	for _, v := range s {
		if !strings.HasPrefix(v, "@") {
			ret = append(ret, v)
		}
	}
	return
}

// CanonicalizeMessage upcases each field in a message slice.
func CanonicalizeMessage(s []string) (ret []string) {
	for _, v := range s {
		ret = append(ret, strings.ToUpper(v))
	}
	return
}

// ExpandAliases takes a string that contains an alias of format "?<alias>" and replaces the alias with the valid ticker string.
func ExpandAliases(ctx context.Context, s []string) ([]string, error) {
	var ret []string
	aliasMap, err := firestorelib.GetAliases(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch aliases: %v", err)
	}
	for _, v := range s {
		if strings.HasPrefix(v, "?") {
			a, ok := aliasMap[v]
			if !ok {
				ret = append(ret, v)
				continue
			}
			ret = append(ret, a...)
			continue
		}
		ret = append(ret, v)
	}
	return ret, nil
}

// DedupeSlice returns a list of unique tickers from the provided string slice.
func DedupeSlice(s []string) (ret []string) {
	seen := make(map[string]bool)
	for _, v := range s {
		if _, exists := seen[v]; !exists {
			seen[v] = true
			ret = append(ret, v)
		}
	}
	return
}
