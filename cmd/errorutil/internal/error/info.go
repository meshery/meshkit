package error

type ErrorInfo struct {
	Name          string `yaml:"name" json:"name"`
	OldCode       string `yaml:"old_code" json:"old_code"`
	Code          string `yaml:"code" json:"code"`
	CodeIsLiteral bool   `yaml:"codeIsLiteral" json:"codeIsLiteral"`
	CodeIsInt     bool   `yaml:"codeIsInt" json:"codeIsInt"`
	Path          string `yaml:"path" json:"path"`
}

type ErrorsInfo struct {
	Entries       []ErrorInfo            `yaml:"entries" json:"entries"`
	LiteralCodes  map[string][]ErrorInfo `yaml:"literalCodes" json:"literalCodes"`
	CallExprCodes []ErrorInfo            `yaml:"callExprCodes" json:"callExprCodes"`
}

func NewErrorsInfo() *ErrorsInfo {
	return &ErrorsInfo{LiteralCodes: make(map[string][]ErrorInfo)}
}
