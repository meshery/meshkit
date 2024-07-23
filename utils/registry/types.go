package registry

import (
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
)

type SpreadsheetData struct {
	Model      *ModelCSV
	Components []v1beta1.ComponentDefinition
}

type ModelCSV struct {
	Registrant         string `json:"registrant" csv:"registrant"`
	ModelDisplayName   string `json:"modelDisplayName" csv:"modelDisplayName"`
	Model              string `json:"model" csv:"model"`
	Category           string `json:"category" csv:"category"`
	SubCategory        string `json:"subCategory" csv:"subCategory"`
	Description        string `json:"description" csv:"description"`
	SourceURL          string `json:"sourceURL" csv:"sourceURL"`
	Website            string `json:"website" csv:"website"`
	Docs               string `json:"docs" csv:"docs"`
	Shape              string `json:"shape" csv:"shape"`
	PrimaryColor       string `json:"primaryColor" csv:"primaryColor"`
	SecondaryColor     string `json:"secondaryColor" csv:"secondaryColor"`
	StyleOverrides     string `json:"styleOverrides" csv:"styleOverrides"`
	Styles             string `json:"styles" csv:"styles"`
	ShapePolygonPoints string `json:"shapePolygonPoints" csv:"shapePolygonPoints"`
	DefaultData        string `json:"defaultData" csv:"defaultData"`
	Capabilities       string `json:"capabilities" csv:"capabilities"`
	LogoURL            string `json:"logoURL" csv:"logoURL"`
	SVGColor           string `json:"svgColor" csv:"svgColor"`
	SVGWhite           string `json:"svgWhite" csv:"svgWhite"`
	SVGComplete        string `json:"svgComplete" csv:"svgComplete"`
	IsAnnotation       string `json:"isAnnotation" csv:"isAnnotation"`
	PublishToRegistry  string `json:"publishToRegistry" csv:"publishToRegistry"`
	AboutProject       string `json:"aboutProject" csv:"-"`
	PageSubtTitle      string `json:"pageSubtitle" csv:"-"`
	DocsURL            string `json:"docsURL" csv:"-"`
	StandardBlurb      string `json:"standardBlurb" csv:"-"`
	Feature1           string `json:"feature1" csv:"-"`
	Feature2           string `json:"feature2" csv:"-"`
	Feature3           string `json:"feature3" csv:"-"`
	HowItWorks         string `json:"howItWorks" csv:"-"`
	HowItWorksDetails  string `json:"howItWorksDetails" csv:"-"`
	Screenshots        string `json:"screenshots" csv:"-"`
	FullPage           string `json:"fullPage" csv:"-"`
	PublishToSites     string `json:"publishToSites" csv:"-"`
}

type ModelCSVHelper struct {
	SpreadsheetID  int64
	SpreadsheetURL string
	Title          string
	CSVPath        string
	Models         []ModelCSV
}

type CompGenerateTracker struct {
	TotalComps int
	Version    string
}