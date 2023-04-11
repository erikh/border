package controlserver

import (
	"fmt"
	"time"

	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/config"
)

func (s *Server) handlePing(req api.Request) (api.Message, error) {
	return req.Response(), nil
}

func (s *Server) handleUptime(req api.Request) (api.Message, error) {
	resp := req.Response().(*api.UptimeResponse)
	resp.Uptime = time.Since(s.bootTime)
	return resp, nil
}

func (s *Server) handleConfigChain(req api.Request) (api.Message, error) {
	resp := req.Response().(*api.ConfigChainResponse)
	resp.Chain = s.config.Chain().AllSums()
	return resp, nil
}

func (s *Server) handleConfigFetch(req api.Request) (api.Message, error) {
	resp := req.Response().(*api.ConfigFetchResponse)
	resp.Config = s.config
	resp.Chain = s.config.Chain().AllSums()

	return resp, nil
}

func (s *Server) handleACMEChallenge(req api.Request) (api.Message, error) {
	resp := req.Response().(*api.ACMEChallengeResponse)

	config.EditMutex.RLock() // ugly
	defer config.EditMutex.RUnlock()

	if s.config.Publisher == nil || s.config.Publisher.Name() != s.me.Name() {
		return nil, fmt.Errorf("This node is not the publisher, does not possess ACME challenge")
	}

	chal, ok := s.config.ACMEChallenges[req.(*api.ACMEChallengeRequest).Domain]
	if !ok {
		return nil, fmt.Errorf("ACME challenge is not ready to be served")
	}

	resp.Challenge = chal
	return resp, nil
}

func (s *Server) handleACMEReady(req api.Request) (api.Message, error) {
	rr := req.(*api.ACMEReadyRequest)

	// FIXME this is very much a faith-based transaction. All things said, a
	// malicious peer could fuck with this.
	//
	// The solution requires a shim earlier on in the request cycle where the
	// peer is looked up for the message decryption process. We'd have to be able
	// to encode the peer into the api.Request, which would be a very good idea,
	// but unavailable at the time of this writing.
	peer, err := s.config.FindPeer(rr.Peer)
	if err != nil {
		return nil, fmt.Errorf("Unable to find peer: %w", err)
	}

	config.EditMutex.Lock() // ugly
	defer config.EditMutex.Unlock()

	s.config.ACMEReady[rr.Domain] = append(s.config.ACMEReady[rr.Domain], peer)

	return req.Response(), nil
}

func (s *Server) handleACMEServe(req api.Request) (api.Message, error) {
	// NOTE resp.Ok will be false by default, per golang rules
	resp := req.Response().(*api.ACMEServeResponse)

	config.EditMutex.RLock() // ugly
	defer config.EditMutex.RUnlock()

	if s.config.Publisher == nil || s.config.Publisher.Name() != s.me.Name() {
		return nil, fmt.Errorf("This node is not the publisher, does not possess ACME challenge")
	}

	peers, ok := s.config.ACMEReady[req.(*api.ACMEChallengeRequest).Domain]
	if !ok {
		return resp, nil
	}

	for _, peer := range s.config.Peers {
		var found bool
		for _, p := range peers {
			if peer.Name() == p.Name() {
				found = true
				break
			}
		}

		if !found {
			return resp, nil
		}
	}

	resp.Ok = true
	return resp, nil
}
