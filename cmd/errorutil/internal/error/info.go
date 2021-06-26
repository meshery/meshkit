package error

type Info struct {
	Name          string `yaml:"name" json:"name"`
	OldCode       string `yaml:"old_code" json:"old_code"`
	Code          string `yaml:"code" json:"code"`
	CodeIsLiteral bool   `yaml:"code_is_literal" json:"code_is_literal"`
	CodeIsInt     bool   `yaml:"code_is_int" json:"code_is_int"`
	Path          string `yaml:"path" json:"path"`
}

type InfoAll struct {
	Entries              []Info             `yaml:"entries" json:"entries"`                                // raw entries
	LiteralCodes         map[string][]Info  `yaml:"literal_codes" json:"literal_codes"`                    // entries with literal codes
	CallExprCodes        []Info             `yaml:"call_expr_codes" json:"call_expr_codes"`                // entries with call expressions
	DeprecatedNewDefault []string           `yaml:"deprecated_new_default" json:"deprecated_new_default" ` // list of files with usage of deprecated NewDefault func
	Errors               map[string][]Error `yaml:"errors_raw" json:"errors_raw"`                          // map of detected errors created using errors.New(...). The key is the error name, more than 1 entry in the list is a duplication error.
}

func NewInfoAll() *InfoAll {
	return &InfoAll{LiteralCodes: make(map[string][]Info), DeprecatedNewDefault: []string{}, CallExprCodes: []Info{}, Errors: map[string][]Error{}}
}
