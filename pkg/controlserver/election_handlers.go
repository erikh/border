package controlserver

import (
	"errors"
	"time"

	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/controlclient"
	"github.com/erikh/border/pkg/election"
)

var (
	ErrElectionIncomplete = errors.New("Election has not completed")
	ErrAllVotesCast       = errors.New("All votes have been cast")
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

	s.election = election.NewElection(election.ElectionContext{
		Config:   s.config,
		Me:       s.me,
		Index:    s.lastVoteIndex + 1,
		BootTime: s.bootTime,
	})

	electoratePeer := s.election.ElectoratePeer()

	s.electoratePeer = electoratePeer
	resp.ElectoratePeer = electoratePeer
	resp.Index = s.election.Index()
	s.lastVoteIndex = s.election.Index()

	return resp, nil
}

func (s *Server) handleRequestVote(req api.Request) (api.Message, error) {
	rvr := req.(*api.RequestVoteRequest)

	peer, err := s.config.FindPeer(rvr.ElectoratePeer)
	if err != nil {
		return nil, err
	}

	// create a temporary election to determine a peer to elect
	election := election.NewElection(election.ElectionContext{
		Config:   s.config,
		Me:       s.me,
		Index:    s.lastVoteIndex + 1,
		BootTime: s.bootTime,
	})

	client := controlclient.FromPeer(peer)
	_, err = client.Exchange(&api.ElectionVoteRequest{Me: s.me.Name(), Peer: election.ElectoratePeer()}, true)

	return req.Response(), err
}

func (s *Server) handleElectionVote(req api.Request) (api.Message, error) {
	evr := req.(*api.ElectionVoteRequest)

	s.electionMutex.Lock()
	defer s.electionMutex.Unlock()

	if s.election == nil || (s.election != nil && s.election.Index() != evr.Index) || s.electoratePeer != s.me.Name() {
		return nil, errors.New("Vote indexes did not match, or electorate was not this instance; is this peer the right electorate?")
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
		if s.config.Publisher != nil && s.lastVoteIndex > 0 {
			resp := req.Response().(*api.IdentifyPublisherResponse)
			resp.EstablishedIndex = s.lastVoteIndex
			resp.Publisher = s.config.Publisher.Name()

			return resp, nil
		} else {
			return nil, errors.New("Election was not completed here")
		}
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

	s.electoratePeer = s.me.Name()
	s.config.SetPublisher(vote)
	s.lastVoteIndex = s.election.Index()

	s.election = nil

	return resp, nil
}
