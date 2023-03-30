package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/josekit"
	"github.com/go-jose/go-jose/v3"
)

const (
	PathNonce             = "nonce"
	PathPeerNonce         = "peerNonce"
	PathAuthCheck         = "authCheck"
	PathPeerRegistration  = "peerRegister"
	PathConfigUpdate      = "configUpdate"
	PathConfigReload      = "configReload"
	PathUptime            = "uptime"
	PathStartElection     = "startElection"
	PathElectionVote      = "electionVote"
	PathIdentifyPublisher = "identifyPublisher"
	PathPing              = "ping"
	PathRequestVote       = "requestVote"
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

type NonceRequest struct{}

func (*NonceRequest) New() Request {
	return &NonceRequest{}
}

func (*NonceRequest) Response() Message {
	return AuthCheck{}
}

func (*NonceRequest) Endpoint() string {
	return PathNonce
}

func (*NonceRequest) Unmarshal(byt []byte) error {
	return nil
}

func (*NonceRequest) Nonce() string {
	return ""
}

func (*NonceRequest) SetNonce(nonce []byte) error {
	return nil
}

func (*NonceRequest) Marshal() ([]byte, error) {
	return []byte("{}"), nil
}

type PeerNonceRequest struct{}

func (*PeerNonceRequest) New() Request {
	return &PeerNonceRequest{}
}

func (*PeerNonceRequest) Response() Message {
	return AuthCheck{}
}

func (*PeerNonceRequest) Endpoint() string {
	return PathPeerNonce
}

func (*PeerNonceRequest) Unmarshal(byt []byte) error {
	return nil
}

func (*PeerNonceRequest) Nonce() string {
	return ""
}

func (*PeerNonceRequest) SetNonce(nonce []byte) error {
	return nil
}

func (*PeerNonceRequest) Marshal() ([]byte, error) {
	return []byte("{}"), nil
}

// both the request for /authCheck, and the response for /nonce
type AuthCheck []byte

func (AuthCheck) New() Request {
	return make(AuthCheck, NonceSize)
}

func (ac AuthCheck) Response() Message {
	return &NilResponse{}
}

func (ac AuthCheck) Endpoint() string {
	return PathAuthCheck
}

func (ac AuthCheck) Unmarshal(byt []byte) error {
	copy(ac, byt)
	return nil
}

func (ac AuthCheck) Nonce() string {
	return string(ac)
}

func (ac AuthCheck) SetNonce(nonce []byte) error {
	return ac.Unmarshal(nonce)
}

func (ac AuthCheck) Marshal() ([]byte, error) {
	return ac, nil
}

type ConfigUpdateRequest struct {
	NonceValue []byte         `json:"nonce"`
	Config     *config.Config `json:"config"`
}

func (*ConfigUpdateRequest) New() Request {
	return &ConfigUpdateRequest{}
}

func (cur *ConfigUpdateRequest) Response() Message {
	return &NilResponse{}
}

func (cur *ConfigUpdateRequest) Endpoint() string {
	return PathConfigUpdate
}

func (cur *ConfigUpdateRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, cur)
}

func (cur *ConfigUpdateRequest) Nonce() string {
	return string(cur.NonceValue)
}

func (cur *ConfigUpdateRequest) SetNonce(nonce []byte) error {
	cur.NonceValue = nonce
	return nil
}

func (cur *ConfigUpdateRequest) Marshal() ([]byte, error) {
	return json.Marshal(cur)
}

type PeerRegistrationRequest struct {
	NonceValue []byte       `json:"nonce"`
	Peer       *config.Peer `json:"peer"`
}

func (*PeerRegistrationRequest) New() Request {
	return &PeerRegistrationRequest{}
}

func (peer *PeerRegistrationRequest) Response() Message {
	return &NilResponse{}
}

func (peer *PeerRegistrationRequest) Endpoint() string {
	return PathPeerRegistration
}

func (peer *PeerRegistrationRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, peer)
}

func (peer *PeerRegistrationRequest) Nonce() string {
	return string(peer.NonceValue)
}

func (peer *PeerRegistrationRequest) SetNonce(nonce []byte) error {
	peer.NonceValue = nonce
	return nil
}

func (peer *PeerRegistrationRequest) Marshal() ([]byte, error) {
	return json.Marshal(peer)
}

type ConfigReloadRequest struct {
	NonceValue []byte `json:"nonce"`
}

func (*ConfigReloadRequest) New() Request {
	return &ConfigReloadRequest{}
}

func (crr *ConfigReloadRequest) Response() Message {
	return &NilResponse{}
}

func (crr *ConfigReloadRequest) Endpoint() string {
	return PathConfigReload
}

func (crr *ConfigReloadRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, crr)
}

func (crr *ConfigReloadRequest) Nonce() string {
	return string(crr.NonceValue)
}

func (crr *ConfigReloadRequest) SetNonce(nonce []byte) error {
	crr.NonceValue = nonce
	return nil
}

func (crr *ConfigReloadRequest) Marshal() ([]byte, error) {
	return json.Marshal(crr)
}

type UptimeRequest struct {
	NonceValue []byte `json:"nonce"`
}

func (*UptimeRequest) New() Request {
	return &UptimeRequest{}
}

func (ur *UptimeRequest) Response() Message {
	return &UptimeResponse{}
}

func (ur *UptimeRequest) Endpoint() string {
	return PathUptime
}

func (ur *UptimeRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, ur)
}

func (ur *UptimeRequest) Nonce() string {
	return string(ur.NonceValue)
}

func (ur *UptimeRequest) SetNonce(nonce []byte) error {
	ur.NonceValue = nonce
	return nil
}

func (ur *UptimeRequest) Marshal() ([]byte, error) {
	return json.Marshal(ur)
}

type UptimeResponse struct {
	Uptime time.Duration `json:"uptime"`
}

func (ur *UptimeResponse) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, ur)
}

func (ur *UptimeResponse) Marshal() ([]byte, error) {
	return json.Marshal(ur)
}

type PingRequest struct {
	NonceValue []byte `json:"nonce"`
}

func (*PingRequest) New() Request {
	return &PingRequest{}
}

func (*PingRequest) Response() Message {
	return &NilResponse{}
}

func (*PingRequest) Endpoint() string {
	return PathPing
}

func (pr *PingRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, pr)
}

func (pr *PingRequest) Nonce() string {
	return string(pr.NonceValue)
}

func (pr *PingRequest) SetNonce(nonce []byte) error {
	pr.NonceValue = nonce
	return nil
}

func (pr *PingRequest) Marshal() ([]byte, error) {
	return json.Marshal(pr)
}
