package launcher

import (
	"context"
	"fmt"
	"time"

	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/controlserver"
	"github.com/erikh/border/pkg/dnsserver"
	"github.com/erikh/border/pkg/lb"
)

type Server struct {
	control   *controlserver.Server
	dns       dnsserver.DNSServer // FIXME make pointer
	balancers []*lb.Balancer
}

func (s *Server) Launch(c config.Config) error {
	cs, err := controlserver.Start(c, c.Listen.Control, controlserver.NonceExpiration, 100*time.Millisecond)
	if err != nil {
		return err
	}

	dnsserver := dnsserver.DNSServer{
		Zones: c.Zones,
	}

	if err := dnsserver.Start(c.Listen.DNS); err != nil {
		return err
	}

	s.dns = dnsserver
	s.control = cs

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.dns.Shutdown(); err != nil {
		return fmt.Errorf("While terminating DNS server: %v", err)
	}

	if err := s.control.Shutdown(ctx); err != nil {
		return fmt.Errorf("While terminating control server: %v", err)
	}

	return nil
}
