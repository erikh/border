package controlserver

import (
	"time"

	"github.com/erikh/border/pkg/api"
)

func (s *Server) handleUptime(req api.Request) (api.Message, error) {
	resp := req.Response().(*api.UptimeResponse)
	resp.Uptime = time.Since(s.bootTime)
	return resp, nil
}
