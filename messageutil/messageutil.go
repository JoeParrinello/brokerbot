package messageutil

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
func CreateMessageEmbed(ticker string, value float32, change float32) *discordgo.MessageEmbed {
	return createMessageEmbedWithPrefix(ticker, value, change, getMessagePrefix())
}

func createMessageEmbedWithPrefix(ticker string, value float32, change float32, prefix string) *discordgo.MessageEmbed {
	mesg := fmt.Sprintf("Latest Quote: $%.2f", value)
	if !math.IsNaN(float64(change)) && change != 0 {
		mesg = fmt.Sprintf("%s (%.2f%%)", mesg, change)
	}
	return &discordgo.MessageEmbed{
		Title:       ticker,
		URL:         fmt.Sprintf("https://www.google.com/search?q=%s", ticker),
		Description: mesg,
		Footer: &discordgo.MessageEmbedFooter{
			Text: prefix,
		},
	}
}

func getMessagePrefix() string {
	if test {
		return messagePrefix
	}
	return ""
}
