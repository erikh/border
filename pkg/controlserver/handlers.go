package controlserver

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/election"
)

// encrypts a nonce with the key. for authentication challenges, it is expected
// that this nonce will be repeated back to a request.
func (s *Server) handleNonce(req api.Request) (api.Message, error) {
	byt := make([]byte, api.NonceSize)

	ok := true
	var nonce string

	// XXX potential to infinite loop; just seems really unlikely.
	for ok {
		if n, err := rand.Read(byt); err != nil || n != api.NonceSize {
			return nil, fmt.Errorf("Invalid entropy read (size: %d, error: %v)", n, err)
		}

		nonce = string(byt)

		s.nonceMutex.RLock()
		_, ok = s.nonces[nonce]
		s.nonceMutex.RUnlock()
	}

	s.nonceMutex.Lock()
	s.nonces[nonce] = time.Now()
	s.nonceMutex.Unlock()

	return api.AuthCheck(nonce), nil
}

func (s *Server) handleAuthCheck(req api.Request) (api.Message, error) {
	return req.Response(), nil
}

func (s *Server) handleConfigUpdate(req api.Request) (api.Message, error) {
	// if we do not do this after encryption, the authkey may change, which will gum up the
	// encryption of the response. Since there is no response this is not an
	// issue in theory, but in practice the encryption will break, rendering the
	// response invalid.
	s.configMutex.Lock()
	oldConfig := s.config
	s.config = req.(*api.ConfigUpdateRequest).Config
	// XXX hack around the lack of JSON serialization for FilenamePrefix
	s.config.FilenamePrefix = oldConfig.FilenamePrefix
	s.configMutex.Unlock()

	if err := s.saveConfig(); err != nil {
		return nil, err
	}

	return req.Response(), nil
}

func (s *Server) handlePeerRegister(req api.Request) (api.Message, error) {
	prr := req.(*api.PeerRegistrationRequest)

	s.configMutex.Lock()
	peers := []*config.Peer{}
	for _, peer := range s.config.Peers {
		if peer.Name() == prr.Peer.Name() {
			peers = append(peers, prr.Peer)
		} else {
			peers = append(peers, peer)
		}
	}
	s.config.Peers = peers
	s.configMutex.Unlock()

	if err := s.saveConfig(); err != nil {
		return nil, err
	}

	return req.Response(), nil
}

func (s *Server) handleConfigReload(req api.Request) (api.Message, error) {
	if err := s.config.Reload(); err != nil {
		return nil, fmt.Errorf("Error reloading configuration: %v", err)
	}

	return req.Response(), nil
}

func (s *Server) handleUptime(req api.Request) (api.Message, error) {
	resp := req.Response().(*api.UptimeResponse)
	resp.Uptime = time.Since(s.bootTime)
	return resp, nil
}

func (s *Server) handleStartElection(req api.Request) (api.Message, error) {
	resp := req.Response().(*api.StartElectionResponse)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.electionMutex.Lock()
	s.election = election.NewElection(ctx, election.ElectionContext{
		Config:   s.config,
		Me:       s.me,
		Index:    s.lastVoteIndex + 1,
		BootTime: s.bootTime,
	})

	electoratePeer, err := s.election.ElectoratePeer()
	if err != nil {
		return nil, fmt.Errorf("Error determining electorate peer: %v", err)
	}

	s.electoratePeer = electoratePeer
	s.electionMutex.Unlock()

	resp.ElectoratePeer = electoratePeer
	resp.Index = s.election.Index()

	return resp, nil
}
