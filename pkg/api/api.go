package api

import (
	"fmt"

	"github.com/erikh/border/pkg/josekit"
	"github.com/go-jose/go-jose/v3"
)

const (
	NonceSize = 128
)

type Message interface {
	Unmarshal([]byte) error
	Marshal() ([]byte, error)
}

type Request interface {
	Message

	New() Request
	Response() Message
	Endpoint() string
	SetNonce([]byte) error
	Nonce() string
}

func EncryptResponse(authKey *jose.JSONWebKey, response Message) ([]byte, error) {
	byt, err := response.Marshal()
	if err != nil {
		return nil, fmt.Errorf("Could not marshal response: %w", err)
	}

	e, err := josekit.GetEncrypter(authKey)
	if err != nil {
		return nil, fmt.Errorf("Could not initialize encrypter: %w", err)
	}

	cipherText, err := e.Encrypt(byt)
	if err != nil {
		return nil, fmt.Errorf("Could not encrypt ciphertext: %w", err)
	}

	serialized, err := cipherText.CompactSerialize()
	if err != nil {
		return nil, fmt.Errorf("Could not serialize JWE: %w", err)
	}

	return []byte(serialized), nil
}

type NilResponse struct{}

func (nr *NilResponse) Marshal() ([]byte, error) {
	return []byte("{}"), nil
}

func (nr *NilResponse) Unmarshal(byt []byte) error {
	return nil
}
