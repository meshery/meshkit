package broker

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
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

func NewEncoding(enc string) (Encoder, error) {
	switch enc {
	case JSONEncoding:
		return NewJSONEncoding()
	// case GobEncoding:
	// 	return NewGobEncoding()
	// case ProtobufEncoding:
	// 	return NewProtobufEncoding()
	default:
		// If the encoding is not supported should we return an error or a default encoding?
		// TODO: Should there be an errors file for broker overall or just implementations of it
		// return nil, ErrUnsupportedEncoding(enc)
		return nil, nil
	}
}

// An empty struct that implements the Encoder interface for JSON encoding
type JSONEncoder struct{}

func NewJSONEncoding() (Encoder, error) {
	return &JSONEncoder{}, nil
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

// An empty struct that implements the Encoder interface for JSON encoding
type GobEncoder struct{}

func NewGobEncoding() (Encoder, error) {
	gob.Register(Message{})
	return &GobEncoder{}, nil
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
