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
