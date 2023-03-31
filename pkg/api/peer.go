package api

import (
	"encoding/json"
	"time"
)

const (
	PathPeerNonce = "peerNonce"
	PathUptime    = "uptime"
	PathPing      = "ping"
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
