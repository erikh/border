package api

import (
	"encoding/json"

	"github.com/mholt/acmez/acme"
)

const (
	PathACMEChallenge = "acmeChallenge"
	PathACMEReady     = "acmeReady"
)

type ACMEChallengeRequest struct {
	NonceValue []byte `json:"nonce"`
	Domain     string `json:"domain"`
}

func (*ACMEChallengeRequest) New() Request {
	return &ACMEChallengeRequest{}
}

func (*ACMEChallengeRequest) Response() Message {
	return &ACMEChallengeResponse{}
}

func (*ACMEChallengeRequest) Endpoint() string {
	return PathACMEChallenge
}

func (acr *ACMEChallengeRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, acr)
}

func (acr *ACMEChallengeRequest) Nonce() string {
	return string(acr.NonceValue)
}

func (acr *ACMEChallengeRequest) SetNonce(nonce []byte) error {
	acr.NonceValue = nonce
	return nil
}

func (acr *ACMEChallengeRequest) Marshal() ([]byte, error) {
	return json.Marshal(acr)
}

type ACMEChallengeResponse struct {
	Challenge acme.Challenge `json:"challenge"`
}

func (acr *ACMEChallengeResponse) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, acr)
}

func (acr *ACMEChallengeResponse) Marshal() ([]byte, error) {
	return json.Marshal(acr)
}

type ACMEReadyRequest struct {
	NonceValue []byte `json:"nonce"`
	Domain     string `json:"domain"`
}

func (*ACMEReadyRequest) New() Request {
	return &ACMEReadyRequest{}
}

func (*ACMEReadyRequest) Response() Message {
	return &NilResponse{}
}

func (*ACMEReadyRequest) Endpoint() string {
	return PathACMEReady
}

func (arr *ACMEReadyRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, arr)
}

func (arr *ACMEReadyRequest) Nonce() string {
	return string(arr.NonceValue)
}

func (arr *ACMEReadyRequest) SetNonce(nonce []byte) error {
	arr.NonceValue = nonce
	return nil
}

func (arr *ACMEReadyRequest) Marshal() ([]byte, error) {
	return json.Marshal(arr)
}
