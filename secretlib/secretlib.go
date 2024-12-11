package secretlib

import (
	"context"
	"log"
	"os"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

// GetSecrets attempts to fetch API keys from the Google Cloud Secret Manager.
func GetSecrets() (bool, string, string, string) {
	success, finnhubKeyPath, discordKeyPath, twelveDataKeyPath := getTokenPaths()
	if !success {
		log.Println("Failed getting the keypaths")
		return false, "", ""
	}
	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		log.Println("Failed creating secret manager client,", err)
		return false, "", ""
	}

	// Build the requests.
	finnhubRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: finnhubKeyPath,
	}
	discordRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: discordKeyPath,
	}
	twelveDataRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: twelveDataKeyPath,
	}

	// Call the API.
	finnhubResult, err := client.AccessSecretVersion(ctx, finnhubRequest)
	if err != nil {
		log.Println("Failed Getting the Finnhub Key", err)
		return false, "", ""
	}
	discordResult, err := client.AccessSecretVersion(ctx, discordRequest)
	if err != nil {
		log.Println("Failed Getting the Discord Key:", err)
		return false, "", ""
	}
	twelveDataResult, err := client.AccessSecretVersion(ctx, twelveDataRequest)
	if err != nil {
		log.Println("Failed Getting the Twelve Data Key:", err)
		return false, "", ""
	}

	log.Println("BrokerBot loaded API keys from SecretManager")
	return true, string(finnhubResult.GetPayload().GetData()), string(discordResult.GetPayload().GetData()), string(twelveDataResult.GetPayload().GetData())
}

func getTokenPaths() (bool, string, string, string) {
	log.Println("Fetching key paths from env files")
	finnhubKeyPath, finnhubPresent := os.LookupEnv("FINNHUB_KEY_PATH")
	discordKeyPath, discordPresent := os.LookupEnv("DISCORD_KEY_PATH")
	twelveDataKeyPath, twelveDataPresent := os.LookupEnv("DISCORD_KEY_PATH")
	return finnhubPresent && discordPresent && twelveDataPresent, finnhubKeyPath, discordKeyPath, twelveDataKeyPath
}
