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
	PathAuthCheck         = "authCheck"
	PathPeerRegistration  = "peerRegister"
	PathConfigUpdate      = "configUpdate"
	PathConfigReload      = "configReload"
	PathUptime            = "uptime"
	PathStartElection     = "startElection"
	PathElectionVote      = "electionVote"
	PathIdentifyPublisher = "identifyPublisher"
)

type Message interface {
	Unmarshal([]byte) error
	Marshal() ([]byte, error)
}

type Request interface {
	Message
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

// both the request for /authCheck, and the response for /nonce
type AuthCheck []byte

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

type StartElectionRequest struct {
	NonceValue []byte `json:"nonce"`
}

func (ser *StartElectionRequest) Response() Message {
	return &StartElectionResponse{}
}

func (ser *StartElectionRequest) Endpoint() string {
	return PathStartElection
}

func (ser *StartElectionRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, ser)
}

func (ser *StartElectionRequest) Nonce() string {
	return string(ser.NonceValue)
}

func (ser *StartElectionRequest) SetNonce(nonce []byte) error {
	ser.NonceValue = nonce
	return nil
}

func (ser *StartElectionRequest) Marshal() ([]byte, error) {
	return json.Marshal(ser)
}

type StartElectionResponse struct {
	ElectoratePeer string `json:"electorate_peer"`
}

func (ser *StartElectionResponse) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, ser)
}

func (ser *StartElectionResponse) Marshal() ([]byte, error) {
	return json.Marshal(ser)
}

type ElectionVoteRequest struct {
	NonceValue []byte        `json:"nonce"`
	Me         string        `json:"me"`
	Uptime     time.Duration `json:"uptime"`
}

func (evr *ElectionVoteRequest) Response() Message {
	return &NilResponse{}
}

func (evr *ElectionVoteRequest) Endpoint() string {
	return PathElectionVote
}

func (evr *ElectionVoteRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, evr)
}

func (evr *ElectionVoteRequest) Nonce() string {
	return string(evr.NonceValue)
}

func (evr *ElectionVoteRequest) SetNonce(nonce []byte) error {
	evr.NonceValue = nonce
	return nil
}

func (evr *ElectionVoteRequest) Marshal() ([]byte, error) {
	return json.Marshal(evr)
}

type IdentifyPublisherRequest struct {
	NonceValue []byte `json:"nonce"`
}

func (ipr *IdentifyPublisherRequest) Response() Message {
	return &IdentifyPublisherResponse{}
}

func (ipr *IdentifyPublisherRequest) Endpoint() string {
	return PathIdentifyPublisher
}

func (ipr *IdentifyPublisherRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, ipr)
}

func (ipr *IdentifyPublisherRequest) Nonce() string {
	return string(ipr.NonceValue)
}

func (ipr *IdentifyPublisherRequest) SetNonce(nonce []byte) error {
	ipr.NonceValue = nonce
	return nil
}

func (ipr *IdentifyPublisherRequest) Marshal() ([]byte, error) {
	return json.Marshal(ipr)
}

type IdentifyPublisherResponse struct {
	Publisher        string `json:"publisher"`
	EstablishedIndex uint   `json:"established_index"`
}

func (ipr *IdentifyPublisherResponse) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, ipr)
}

func (ipr *IdentifyPublisherResponse) Marshal() ([]byte, error) {
	return json.Marshal(ipr)
}
