package api

import (
	"encoding/json"
	"fmt"

	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/josekit"
	"github.com/go-jose/go-jose/v3"
)

const (
	PathNonce            = "nonce"
	PathAuthCheck        = "authCheck"
	PathPeerRegistration = "peerRegister"
	PathConfigUpdate     = "configUpdate"
)

type Message interface {
	Unmarshal([]byte) error
	SetNonce([]byte)
	Marshal() ([]byte, error)
	Nonce() string
}

type NilResponse struct{}

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

func (nr NilResponse) SetNonce(nonce []byte) {}

func (nr NilResponse) Marshal() ([]byte, error) {
	return []byte("{}"), nil
}

func (nr NilResponse) Unmarshal(byt []byte) error {
	return nil
}

func (nr NilResponse) Nonce() string {
	return ""
}

// both the request for /authCheck, and the response for /nonce
type AuthCheck []byte

func (ac AuthCheck) Unmarshal(byt []byte) error {
	copy(ac, byt)
	return nil
}

func (ac AuthCheck) Nonce() string {
	return string(ac)
}

func (ac AuthCheck) SetNonce(nonce []byte) {
	ac.Unmarshal(nonce)
}

func (ac AuthCheck) Marshal() ([]byte, error) {
	return ac, nil
}

type ConfigUpdateRequest struct {
	NonceValue []byte        `json:"nonce"`
	Config     config.Config `json:"config"`
}

func (cur *ConfigUpdateRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, cur)
}

func (cur *ConfigUpdateRequest) Nonce() string {
	return string(cur.NonceValue)
}

func (cur *ConfigUpdateRequest) SetNonce(nonce []byte) {
	cur.NonceValue = nonce
}

func (cur *ConfigUpdateRequest) Marshal() ([]byte, error) {
	return json.Marshal(cur)
}

type PeerRegistrationRequest struct {
	NonceValue []byte      `json:"nonce"`
	Name       string      `json:"name"`
	Peer       config.Peer `json:"peer"`
}

func (peer *PeerRegistrationRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, peer)
}

func (peer *PeerRegistrationRequest) Nonce() string {
	return string(peer.NonceValue)
}

func (peer *PeerRegistrationRequest) SetNonce(nonce []byte) {
	peer.NonceValue = nonce
}

func (peer *PeerRegistrationRequest) Marshal() ([]byte, error) {
	return json.Marshal(peer)
}
