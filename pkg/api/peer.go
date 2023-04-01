package api

import (
	"encoding/json"
	"time"

	"github.com/erikh/border/pkg/config"
)

const (
	PathPeerNonce   = "peerNonce"
	PathUptime      = "uptime"
	PathPing        = "ping"
	PathConfigChain = "configChain"
	PathConfigFetch = "configFetch"
)

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

type UptimeRequest struct {
	NonceValue []byte `json:"nonce"`
}

func (*UptimeRequest) New() Request {
	return &UptimeRequest{}
}

func (*UptimeRequest) Response() Message {
	return &UptimeResponse{}
}

func (*UptimeRequest) Endpoint() string {
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

type ConfigChainRequest struct {
	NonceValue []byte `json:"nonce"`
}

func (*ConfigChainRequest) New() Request {
	return &ConfigChainRequest{}
}

func (*ConfigChainRequest) Response() Message {
	return &ConfigChainResponse{}
}

func (*ConfigChainRequest) Endpoint() string {
	return PathConfigChain
}

func (cr *ConfigChainRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, cr)
}

func (cr *ConfigChainRequest) Nonce() string {
	return string(cr.NonceValue)
}

func (cr *ConfigChainRequest) SetNonce(nonce []byte) error {
	cr.NonceValue = nonce
	return nil
}

func (cr *ConfigChainRequest) Marshal() ([]byte, error) {
	return json.Marshal(cr)
}

type ConfigChainResponse struct {
	NonceValue []byte   `json:"nonce"`
	Chain      []string `json:"chain"`
}

func (cr *ConfigChainResponse) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, cr)
}

func (cr *ConfigChainResponse) Marshal() ([]byte, error) {
	return json.Marshal(cr)
}

type ConfigFetchRequest struct {
	NonceValue []byte `json:"nonce"`
}

func (*ConfigFetchRequest) New() Request {
	return &ConfigFetchRequest{}
}

func (*ConfigFetchRequest) Response() Message {
	return &ConfigFetchResponse{}
}

func (*ConfigFetchRequest) Endpoint() string {
	return PathConfigFetch
}

func (cr *ConfigFetchRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, cr)
}

func (cr *ConfigFetchRequest) Nonce() string {
	return string(cr.NonceValue)
}

func (cr *ConfigFetchRequest) SetNonce(nonce []byte) error {
	cr.NonceValue = nonce
	return nil
}

func (cr *ConfigFetchRequest) Marshal() ([]byte, error) {
	return json.Marshal(cr)
}

type ConfigFetchResponse struct {
	NonceValue []byte         `json:"nonce"`
	Config     *config.Config `json:"config"`
	Chain      []string       `json:"chain"`
}

func (cr *ConfigFetchResponse) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, cr)
}

func (cr *ConfigFetchResponse) Marshal() ([]byte, error) {
	return json.Marshal(cr)
}
