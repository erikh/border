package controlserver

import (
	"github.com/erikh/border/pkg/api"
)

func (s *Server) handlePing(req api.Request) (api.Message, error) {
	return req.Response(), nil
}
