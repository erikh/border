package controlserver

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/election"
)

var (
	ErrElectionIncomplete = errors.New("Election has not completed")
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

	s.electionMutex.Lock()
	defer s.electionMutex.Unlock()

	if s.election != nil && s.election.Index() == s.lastVoteIndex && !s.election.Voter().ReadyToVote() {
		return nil, ErrElectionIncomplete
	}

	s.election = election.NewElection(election.ElectionContext{
		Config:   s.config,
		Me:       s.me,
		Index:    s.lastVoteIndex + 1,
		BootTime: s.bootTime,
	})

	electoratePeer, err := s.election.ElectoratePeer()
	if err != nil {
		return nil, fmt.Errorf("Error determining electorate peer: %w", err)
	}

	s.electoratePeer = electoratePeer
	resp.ElectoratePeer = electoratePeer
	resp.Index = s.election.Index()
	s.lastVoteIndex = s.election.Index()

	return resp, nil
}

func (s *Server) handleElectionVote(req api.Request) (api.Message, error) {
	evr := req.(*api.ElectionVoteRequest)

	s.electionMutex.Lock()
	defer s.electionMutex.Unlock()

	if s.election == nil || (s.election != nil && s.election.Index() != evr.Index) || s.electoratePeer != s.me.Name() {
		return nil, errors.New("Vote indexes did not match, or electorate was not this instance; is this peer the right electorate?")
	}

	if s.election.Voter().ReadyToVote() {
		return nil, errors.New("All votes have been cast")
	}

	me, err := s.config.FindPeer(evr.Me)
	if err != nil {
		return nil, err
	}

	peer, err := s.config.FindPeer(evr.Peer)
	if err != nil {
		return nil, err
	}

	s.election.RegisterVote(me, peer)
	return req.Response(), nil
}

func (s *Server) handleIdentifyPublisher(req api.Request) (api.Message, error) {
	s.electionMutex.Lock()
	defer s.electionMutex.Unlock()

	if s.election == nil {
		return nil, errors.New("Peer has never held an election")
	}

	if !s.election.Voter().ReadyToVote() {
		return nil, errors.New("Voter is not ready to vote")
	}

	vote, err := s.election.Voter().Vote()
	if err != nil {
		return nil, err
	}

	resp := req.Response().(*api.IdentifyPublisherResponse)
	resp.EstablishedIndex = s.election.Index()
	resp.Publisher = vote.Name()

	return resp, nil
}
