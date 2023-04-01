package controlserver

import (
	"time"

	"github.com/erikh/border/pkg/api"
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
