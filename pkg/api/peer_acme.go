package api

import (
	"encoding/json"

	"github.com/mholt/acmez/acme"
)

const (
	PathACMEChallenge         = "acmeChallenge"
	PathACMEReady             = "acmeReady"
	PathACMEServe             = "acmeServe"
	PathACMEChallengeComplete = "acmeChallengeComplete"
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
	Peer       string `json:"peer"`
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

type ACMEServeRequest struct {
	NonceValue []byte `json:"nonce"`
	Domain     string `json:"domain"`
}

func (*ACMEServeRequest) New() Request {
	return &ACMEServeRequest{}
}

func (*ACMEServeRequest) Response() Message {
	return &ACMEServeResponse{}
}

func (*ACMEServeRequest) Endpoint() string {
	return PathACMEServe
}

func (asr *ACMEServeRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, asr)
}

func (asr *ACMEServeRequest) Nonce() string {
	return string(asr.NonceValue)
}

func (asr *ACMEServeRequest) SetNonce(nonce []byte) error {
	asr.NonceValue = nonce
	return nil
}

func (asr *ACMEServeRequest) Marshal() ([]byte, error) {
	return json.Marshal(asr)
}

type ACMEServeResponse struct {
	Ok bool `json:"ok"`
}

func (asr *ACMEServeResponse) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, asr)
}

func (asr *ACMEServeResponse) Marshal() ([]byte, error) {
	return json.Marshal(asr)
}

type ACMEChallengeCompleteRequest struct {
	NonceValue []byte `json:"nonce"`
	Domain     string `json:"domain"`
}

func (*ACMEChallengeCompleteRequest) New() Request {
	return &ACMEChallengeCompleteRequest{}
}

func (*ACMEChallengeCompleteRequest) Response() Message {
	return &ACMEChallengeCompleteResponse{}
}

func (*ACMEChallengeCompleteRequest) Endpoint() string {
	return PathACMEChallengeComplete
}

func (accr *ACMEChallengeCompleteRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, accr)
}

func (accr *ACMEChallengeCompleteRequest) Nonce() string {
	return string(accr.NonceValue)
}

func (accr *ACMEChallengeCompleteRequest) SetNonce(nonce []byte) error {
	accr.NonceValue = nonce
	return nil
}

func (accr *ACMEChallengeCompleteRequest) Marshal() ([]byte, error) {
	return json.Marshal(accr)
}

type ACMEChallengeCompleteResponse struct {
	// Indicate whether the challenge was actually completed.
	Ok bool `json:"ok"`

	// some confused and poorly educated nerd is going to eventually ask why we
	// share both of these.
	//
	// first off, this entire transaction is encrypted even if it is served over
	// HTTP. second, without the private key, the certificate is useless to the
	// peer. each peer would either have to generate its own private key or share
	// it at some point, which would have just moved the problem to a spot where
	// it was more complicated to handle. Even though ACME's endpoints never see
	// the private key, it is required to operate with the certificate ACME would
	// produce. Since the challenge will not go to all nodes, just one, we must
	// account for that.
	//
	// FIXME There is room for improvement in the JOSE exchange, which is currently
	// using symmetric keys. This is not the problem of this API request.
	Chain      []byte `json:"chain"`
	PrivateKey []byte `json:"private_key"`
}

func (accr *ACMEChallengeCompleteResponse) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, accr)
}

func (accr *ACMEChallengeCompleteResponse) Marshal() ([]byte, error) {
	return json.Marshal(accr)
}
