package drive

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

// loadApiConfig lädt das angegebene client_secret.json und erstellt daraus das config Objekt.
// Bei einem Fehler wird eine Anleitung zum Erstellen einer client secret Datei auf der Konsole ausgegeben.
// ACHTUNG: Die Methode terminiert (os.exit) im Fehlerfall mit 11 ... file not found, oder 12 ... parsing error!
func loadApiConfig(clientSecretPath string, withWriteAccess bool) *oauth2.Config {

	// error info text
	const createClientSecretMsg = `
ERROR: %v

Please create a new Drive API!
(Tutorial: https://developers.google.com/drive/api/v3/quickstart/go)

 1) Use this wizard (https://console.developers.google.com/start/api?id=drive)
    to create or select a project in the Google Developers Console and
    automatically turn on the API. Click Continue, then Go to credentials.
 2) On the Add credentials to your project page, click the Cancel button.
 3) At the top of the page, select the OAuth consent screen tab.
    Select an Email address, enter a Product name if not already set, and click
    the Save button.
 4) Select the Credentials tab, click the Create credentials button and select
    OAuth client ID.
 5) Select the application type Other, enter the name "SplitFuse X", and click
    the Create button.
 6) Click OK to dismiss the resulting dialog.
 7) Click the Download JSON button to the right of the client ID.
 8) Move this file to your working directory and save it to the file
    '%s'.
 9) Restart the program.
`
	// scope
	//   normal: DriveReadonlyScope
	//   for uploads: DriveScope
	scope := drive.DriveReadonlyScope
	if withWriteAccess {
		scope = drive.DriveScope
	}

	// read client_secret json file
	bytes, err := ioutil.ReadFile(clientSecretPath)
	if err != nil {
		fmt.Printf(createClientSecretMsg, err, clientSecretPath)
		os.Exit(11)
	}

	// parse config object
	config, err := google.ConfigFromJSON(bytes, scope)
	if err != nil {
		fmt.Printf(createClientSecretMsg, err, clientSecretPath)
		os.Exit(12)
	}

	// ok -> return config
	return config
}

// loadToken lädt den Token für den Zugriff auf Google Drive aus der Datei.
// Bei einem Fehler wird eine Aufforderung zum Erstellen eines Tokens auf der Konsole ausgegeben.
// ACHTUNG: Die Methode terminiert (os.exit) im Fehlerfall mit 21 ... file not found, oder 22 ... parsing error!
func loadToken(tokenFilePath string) *oauth2.Token {

	// read token file
	fh, err := os.Open(tokenFilePath)
	if err != nil {
		fmt.Printf("ERROR: %v\n\nCreate a new token file!\n", err)
		os.Exit(21)
	}
	defer fh.Close()

	// parse token json
	tok := &oauth2.Token{}
	err = json.NewDecoder(fh).Decode(tok)
	if err != nil {
		fmt.Printf("ERROR: %v\n\nCreate a new token file!\n", err)
		os.Exit(22)
	}

	// ok -> return access token
	return tok
}

// CreateTokenFile interagiert auf der Konsole mit dem Benutzer und erstellt einen neues oauth token file.
// Bei einem Fehler terminiert das Programm mit os.exit und leitet den Benutzer mit einer Fehlermeldung an (siehe: loadApiConfig()).
// Fehlercode: 11, 12, 21, 22, 31, 32 und 33
func CreateTokenFile(clientSecretPath string, tokenFilePath string, withWriteAccess bool) {

	// error info text
	const createTokenMsg = `
Create a new oauth access token with %s access!
Write token to file: %s

Go to the following link in your browser then type the authorization code: 
%v
--------------------------------------------------------------------------
`
	// load config  (exit on error)
	config := loadApiConfig(clientSecretPath, withWriteAccess)

	// print auth url
	accessType := "READONLY"
	if withWriteAccess {
		accessType = "READ & WRITE"
	}
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf(createTokenMsg, accessType, tokenFilePath, authURL)

	// read authCode (user input)
	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		fmt.Printf("unable to read authorization code: %v", err)
		os.Exit(31)
	}

	// generate token
	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		fmt.Printf("unable to retrieve token from web: %v", err)
		os.Exit(32)
	}

	// save token to file
	f, err := os.OpenFile(tokenFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Printf("unable to write oauth token file: %v", err)
		os.Exit(33)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(tok)
}
