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
	Entries       []Info            `yaml:"entries" json:"entries"`
	LiteralCodes  map[string][]Info `yaml:"literal_codes" json:"literal_codes"`
	CallExprCodes []Info            `yaml:"call_expr_codes" json:"call_expr_codes"`
}

func NewInfoAll() *InfoAll {
	return &InfoAll{LiteralCodes: make(map[string][]Info)}
}
