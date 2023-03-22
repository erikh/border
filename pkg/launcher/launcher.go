package launcher

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/controlserver"
	"github.com/erikh/border/pkg/dnsconfig"
	"github.com/erikh/border/pkg/dnsserver"
	"github.com/erikh/border/pkg/lb"
)

type Server struct {
	control   *controlserver.Server
	dns       dnsserver.DNSServer // FIXME make pointer
	balancers []*lb.Balancer
}

func (s *Server) Launch(peerName string, c *config.Config) error {
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

	balancers, err := s.createBalancers(peerName, c)
	if err != nil {
		return err
	}

	s.dns = dnsserver
	s.control = cs
	s.balancers = balancers

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.dns.Shutdown(); err != nil {
		return fmt.Errorf("While terminating DNS server: %v", err)
	}

	if err := s.control.Shutdown(ctx); err != nil {
		return fmt.Errorf("While terminating control server: %v", err)
	}

	for _, balancer := range s.balancers {
		balancer.Shutdown()
	}

	return nil
}

func (s *Server) createBalancers(peerName string, c *config.Config) ([]*lb.Balancer, error) {
	balancers := []*lb.Balancer{}

	for _, zone := range c.Zones {
		for _, rec := range zone.Records {
			if rec.Type == dnsconfig.TypeLB {
				lbRecord, ok := rec.Value.(*dnsconfig.LB)
				if !ok {
					return nil, fmt.Errorf("LB record for %q was not parsed correctly", rec.Name)
				}

				for _, listener := range lbRecord.Listeners {
					fmt.Println(listener)
					host, _, err := net.SplitHostPort(listener)
					if err != nil {
						return nil, fmt.Errorf("Invalid listener %q: could not parse: %v", listener, err)
					}

					if host == c.Peers[peerName].IP.String() {
						bc := lb.BalancerConfig{
							Kind:                     lbRecord.Kind,
							Backends:                 lbRecord.Backends,
							SimultaneousConnections:  lbRecord.SimultaneousConnections,
							MaxConnectionsPerAddress: lbRecord.MaxConnectionsPerAddress,
							ConnectionTimeout:        lbRecord.ConnectionTimeout,
						}

						balancer := lb.Init(listener, bc)
						if err := balancer.Start(); err != nil {
							return nil, fmt.Errorf("Could not start balancer %q: %v", rec.Name, err)
						}

						balancers = append(balancers, balancer)
					}
				}
			}
		}
	}

	return balancers, nil
}
