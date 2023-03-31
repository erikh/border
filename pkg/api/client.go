package api

import (
	"encoding/json"

	"github.com/erikh/border/pkg/config"
)

const (
	PathNonce             = "nonce"
	PathAuthCheck         = "authCheck"
	PathPeerRegistration  = "peerRegister"
	PathConfigUpdate      = "configUpdate"
	PathConfigReload      = "configReload"
	PathIdentifyPublisher = "identifyPublisher"
)

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

// both the request for /authCheck, and the response for /nonce
type AuthCheck []byte

func (AuthCheck) New() Request {
	return make(AuthCheck, NonceSize)
}

func (AuthCheck) Response() Message {
	return &NilResponse{}
}

func (AuthCheck) Endpoint() string {
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

func (*ConfigUpdateRequest) Response() Message {
	return &NilResponse{}
}

func (*ConfigUpdateRequest) Endpoint() string {
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

func (*PeerRegistrationRequest) Response() Message {
	return &NilResponse{}
}

func (*PeerRegistrationRequest) Endpoint() string {
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

func (*ConfigReloadRequest) Response() Message {
	return &NilResponse{}
}

func (*ConfigReloadRequest) Endpoint() string {
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

type IdentifyPublisherRequest struct {
	NonceValue []byte `json:"nonce"`
}

func (*IdentifyPublisherRequest) New() Request {
	return &IdentifyPublisherRequest{}
}

func (*IdentifyPublisherRequest) Response() Message {
	return &IdentifyPublisherResponse{}
}

func (*IdentifyPublisherRequest) Endpoint() string {
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
	NonceValue []byte `json:"nonce"`
	Publisher  string `json:"publisher"`
}

func (ipr *IdentifyPublisherResponse) Marshal() ([]byte, error) {
	return json.Marshal(ipr)
}

func (ipr *IdentifyPublisherResponse) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, ipr)
}
