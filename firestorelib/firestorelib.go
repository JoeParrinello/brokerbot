package firestorelib

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/JoeParrinello/brokerbot/shutdownlib"
	"google.golang.org/api/option"
)

var (
	cloudPlatformProjectId     = flag.String("project", "", "Google Cloud Platform Project ID")
	credentialsFilePath        = flag.String("credentials_file", "credentials/credentials.json", "Google Cloud Platform Credentials File")
	firestoreClient            *firestore.Client
	firestoreAliasesCollection = "aliases"
	firestoreConnected         bool
)

func Init() {
	ctx := context.Background()
	var err error

	// If we're running on Cloud Run, we can connect automatically.
	firestoreClient, err = connectWithImplicitCredentials(ctx)
	if err != nil {
		log.Println(err)

		// If we're running locally, we need to use a credential file.
		log.Println("attempting to use local credentials for Firestore")
		firestoreClient, err = connectWithExplicitCredentials(ctx)
		if err != nil {
			log.Println(err)

			// Continue without database support.
			firestoreConnected = false
			return
		}
	}

	firestoreConnected = true

	shutdownlib.AddShutdownHandler(func() error {
		log.Printf("BrokerBot shutting down connection to Firestore.")
		return firestoreClient.Close()
	})
}

func connectWithImplicitCredentials(ctx context.Context) (*firestore.Client, error) {
	firestoreClient, err := firestore.NewClient(ctx, *cloudPlatformProjectId)
	if err != nil {
		return nil, fmt.Errorf("failed to create Firestore Client with implicit credentials: %v", err)
	}
	return firestoreClient, nil
}

func connectWithExplicitCredentials(ctx context.Context) (*firestore.Client, error) {
	firestoreClient, err := firestore.NewClient(ctx, *cloudPlatformProjectId, option.WithCredentialsFile(*credentialsFilePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create Firestore Client with explicit credentials: %v", err)
	}
	return firestoreClient, nil
}

func GetAliases(ctx context.Context) (map[string][]string, error) {
	if !firestoreConnected {
		return nil, errors.New("firestore not connected")
	}

	documents, err := getDocumentsInCollection(ctx, firestoreAliasesCollection)
	if err != nil {
		return nil, err
	}

	aliasMap := make(map[string][]string)
	for _, doc := range documents {
		alias := strings.ToUpper(doc["alias"].(string))
		assets := interfaceSliceToStringSlice(doc["assets"].([]interface{}))
		aliasMap[alias] = assets
	}
	return aliasMap, nil
}

func interfaceSliceToStringSlice(islice []interface{}) (ret []string) {
	for _, v := range islice {
		ret = append(ret, strings.ToUpper(v.(string)))
	}
	return
}

func CreateAlias(ctx context.Context, alias string, assets []string) error {
	if !firestoreConnected {
		return errors.New("firestore not connected")
	}

	_, _, err := firestoreClient.Collection(firestoreAliasesCollection).Add(ctx, map[string]interface{}{
		"alias":  alias,
		"assets": assets,
	})
	if err != nil {
		return fmt.Errorf("failed to create alias: %v", err)
	}

	return nil
}

func GetAlias(ctx context.Context, alias string) ([]string, error) {
	if !firestoreConnected {
		return nil, errors.New("firestore not connected")
	}

	docs, err := firestoreClient.Collection(firestoreAliasesCollection).Where("alias", "==", strings.ToUpper(alias)).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to find aliases: %v", err)
	}

	for _, doc := range docs {
		if doc.Data()["alias"] == strings.ToUpper(alias) {
			return interfaceSliceToStringSlice(doc.Data()["assets"].([]interface{})), nil
		}
	}

	return nil, fmt.Errorf("alias %q not found", alias)
}

func DeleteAlias(ctx context.Context, alias string) error {
	if !firestoreConnected {
		return errors.New("firestore not connected")
	}

	docs, err := firestoreClient.Collection(firestoreAliasesCollection).Where("alias", "==", alias).Documents(ctx).GetAll()
	if err != nil {
		return fmt.Errorf("failed to find aliases: %v", err)
	}

	for _, doc := range docs {
		if doc.Data()["alias"] == alias {
			doc.Ref.Delete(ctx)
			return nil
		}
	}

	return nil
}

func getDocumentsInCollection(ctx context.Context, collection string) ([]map[string]interface{}, error) {
	if !firestoreConnected {
		return nil, errors.New("firestore not connected")
	}

	docs, err := firestoreClient.Collection(collection).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to find documents: %v", err)
	}

	documents := make([]map[string]interface{}, 0)
	for _, doc := range docs {
		documents = append(documents, doc.Data())
	}

	return documents, nil
}
