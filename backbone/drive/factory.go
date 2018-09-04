package drive

import (
	"fmt"
	"os"

	"splitfuseX/backbone"

	"golang.org/x/net/context"
	"google.golang.org/api/drive/v3"
)

// NewApiClient gibt einen drive api client zurück.
// Die folderId gibt den Ordner mit den Chunks an (default ist root).
// Im Fehlerfall terminiert das Programm mit os.Exit (siehe loadApiConfig() und loadToken())
// HINWEIS: Bleibt der cachePath leer, dann ist diese Funktionalität deaktiviert!
func NewApiClient(clientSecretPath, tokenFilePath, cachePath, folderId string) backbone.Client {

	// load client_secret file
	config := loadApiConfig(clientSecretPath, false)

	// load token file
	token := loadToken(tokenFilePath)

	// create client
	client := config.Client(context.Background(), token)

	// create drive service (API)
	api, err := drive.New(client)
	if err != nil {
		fmt.Printf("unable to retrieve Drive client: %v", err)
		os.Exit(41)
	}

	// return
	var ret *ApiClient
	ret = &ApiClient{api: api, folderId: folderId, cachePath: cachePath}
	return ret
}
