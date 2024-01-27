package utils

import (
	"context"
	"encoding/base64"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var (
	GoogleSpreadSheetURL  = "https://docs.google.com/spreadsheets/d/"
)

func NewSheetSRV(cred string) (*sheets.Service, error) {
	ctx := context.Background()
	byt, _ := base64.StdEncoding.DecodeString(cred)
	// authenticate and get configuration
	config, err := google.JWTConfigFromJSON(byt, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		return nil, err
	}
	// create client with config and context
	client := config.Client(ctx)
	// create new service using client
	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	return srv, nil
}