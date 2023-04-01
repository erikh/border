package controlserver

import (
	"fmt"

	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/config"
)

func (s *Server) handleAuthCheck(req api.Request) (api.Message, error) {
	return req.Response(), nil
}

func (s *Server) handleConfigUpdate(req api.Request) (api.Message, error) {
	newConfig := req.(*api.ConfigUpdateRequest).Config
	return req.Response(), s.ReplaceConfig(newConfig, s.config.Chain())
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

func (s *Server) handleIdentifyPublisher(req api.Request) (api.Message, error) {
	publisher := s.config.GetPublisher()

	resp := req.Response().(*api.IdentifyPublisherResponse)
	resp.Publisher = publisher.Name()
	return resp, nil
}
