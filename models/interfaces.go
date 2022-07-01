package models

// anything that can be validated is a Validator
type Validator interface {
	Validate([]byte) error
}
