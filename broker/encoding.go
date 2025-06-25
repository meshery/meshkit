package broker

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"strings"
)

const (
	JSONEncoding     = "json"
	GobEncoding      = "gob"
	ProtobufEncoding = "protobuf"
)

type Encoder interface {
	Encode(v any) ([]byte, error)
	Decode(data []byte, v any) error
	Encoding() string
}

func NewEncoding(enc string) Encoder {
	switch strings.ToLower(enc) {
	case GobEncoding:
		return NewGobEncoding()
	// case ProtobufEncoding:
	// 	return NewProtobufEncoding()
	default:
		return NewJSONEncoding()
	}
}

// An empty struct that implements the Encoder interface for JSON encoding
type JSONEncoder struct{}

func NewJSONEncoding() *JSONEncoder {
	return &JSONEncoder{}
}

func (j *JSONEncoder) Encode(v any) ([]byte, error) {
	enc, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return enc, err
}

func (j *JSONEncoder) Decode(data []byte, v any) error {
	err := json.Unmarshal(data, v)
	return err
}

func (j *JSONEncoder) Encoding() string {
	return JSONEncoding
}

type GobEncoder struct{}

func NewGobEncoding() *GobEncoder {
	gob.Register(Message{})
	return &GobEncoder{}
}

func (g *GobEncoder) Encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(v)

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (g *GobEncoder) Decode(data []byte, v any) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(v)

	if err != nil {
		return err
	}

	return nil
}

func (g *GobEncoder) Encoding() string {
	return GobEncoding
}
