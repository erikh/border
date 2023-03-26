package controlserver

import (
	"errors"
	"fmt"
	"time"

	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/election"
)

var (
	ErrElectionIncomplete = errors.New("Election has not completed")
)

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
