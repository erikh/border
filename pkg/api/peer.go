package api

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
