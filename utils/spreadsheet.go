package utils

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var NameToIndex = map[string]int{ //Update this on addition of new columns
	"modelDisplayName":  0,
	"model":             1,
	"category":          2,
	"subCategory":       3,
	"CRDs":              4,
	"link":              5,
	"hasSchema?":        6,
	"component":         7,
	"shape":             8,
	"primaryColor":      9,
	"secondaryColor":    10,
	"styleOverrides":    11,
	"logoURL":           12,
	"svgColor":          13,
	"svgWhite":          14,
	"svgComplete":       15,
	"genealogy":         16,
	"About Project":     17,
	"Page Subtitle":     18,
	"Docs URL":          19,
	"Standard Blurb":    20,
	"Feature 1":         21,
	"Feature 2":         22,
	"Feature 3":         23,
	"howItWorks":        24,
	"howItWorksDetails": 25,
	"Screenshots":       26,
	"Full Page":         27,
	"Publish?":          28,
}



func Spreadsheet(
	srv *sheets.Service, 
	sheetName, 
	spreadsheetID string, 
	modelChan chan v1alpha1.ModelChannel, 
	am map[string][]interface{}, 
	acpm map[string]map[string]bool) {
	start := time.Now()
	rangeString := sheetName + "!A4:AB4"
	// Get the value of the specified cell.
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, rangeString).Do()
	if err != nil {
		fmt.Println("Unable to retrieve data from sheet: ", err)
		return
	}
	batchSize := 100
	values := make([][]interface{}, 0)
	for entry := range modelChan {
		if len(entry.Comps) == 0 {
			continue
		}
		for _, comp := range entry.Comps {
			if acpm[entry.Model][comp.Kind] {
				fmt.Println("[Debug][Spreadsheet] Skipping spreadsheet updation for ", entry.Model, comp.Kind)
				continue
			}
			var newValues []interface{}
			if am[entry.Model] != nil {
				newValues = make([]interface{}, len(am[entry.Model]))
				copy(newValues, am[entry.Model])
			} else {
				newValues = make([]interface{}, len(resp.Values[0]))
				copy(newValues, resp.Values[0])
				newValues[NameToIndex["modelDisplayName"]] = entry.Model
				newValues[NameToIndex["model"]] = entry.Model
			}
			newValues[NameToIndex["component"]] = comp.Kind
			if comp.Schema != "" {
				newValues[NameToIndex["hasSchema?"]] = true
			} else {
				newValues[NameToIndex["hasSchema?"]] = false
			}
			newValues[NameToIndex["link"]] = entry.HelmURL
			values = append(values, newValues)
			if acpm[entry.Model] == nil {
				acpm[entry.Model] = make(map[string]bool)
			}
			acpm[entry.Model][comp.Kind] = true
			batchSize--
			fmt.Println("Batch size: ", batchSize)
			if batchSize <= 0 {
				row := &sheets.ValueRange{
					Values: values,
				}
				response2, err := srv.Spreadsheets.Values.Append(spreadsheetID, sheetName, row).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Context(context.Background()).Do()
				values = make([][]interface{}, 0)
				batchSize = 100
				if err != nil || response2.HTTPStatusCode != 200 {
					fmt.Println(err)
					continue
				}
			}
		}
		if am[entry.Model] != nil {
			fmt.Println("[Debug][Spreadsheet] Skipping spreadsheet updation for ", entry.Model)
			continue
		}
		newValues := make([]interface{}, len(resp.Values[0]))
		copy(newValues, resp.Values[0])
		newValues[NameToIndex["modelDisplayName"]] = entry.Model
		newValues[NameToIndex["model"]] = entry.Model
		newValues[NameToIndex["CRDs"]] = len(entry.Comps)
		newValues[NameToIndex["link"]] = entry.HelmURL
		values = append(values, newValues)
		copy(am[entry.Model], newValues)
		batchSize--
		fmt.Println("Batch size: ", batchSize)
		if batchSize <= 0 {
			row := &sheets.ValueRange{
				Values: values,
			}
			response2, err := srv.Spreadsheets.Values.Append(spreadsheetID, sheetName, row).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Context(context.Background()).Do()
			values = make([][]interface{}, 0)
			batchSize = 100
			if err != nil || response2.HTTPStatusCode != 200 {
				fmt.Println(err)
				continue
			}
		}
	}
	if len(values) != 0 {
		row := &sheets.ValueRange{
			Values: values,
		}
		response2, err := srv.Spreadsheets.Values.Append(spreadsheetID, sheetName, row).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Context(context.Background()).Do()
		if err != nil || response2.HTTPStatusCode != 200 {
			fmt.Println(err)
		}
	}
	elapsed := time.Now().Sub(start)
	fmt.Printf("Time taken by spreadsheet updater in minutes (including the time it required to generate components): %f", elapsed.Minutes())
}

func NewSheetSRV() *sheets.Service {
	ctx := context.Background()
	byt, _ := base64.StdEncoding.DecodeString(os.Getenv("CRED")) // TODO: remove the requirement of CRED and take input from mesheryctl
	// authenticate and get configuration
	config, err := google.JWTConfigFromJSON(byt, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		fmt.Println("ERR2", err)
		return nil
	}
	// create client with config and context
	client := config.Client(ctx)
	// create new service using client
	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		fmt.Println("ERR3", err)
		return nil
	}
	return srv
}
