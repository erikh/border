package launcher

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/controlserver"
	"github.com/erikh/border/pkg/dnsconfig"
	"github.com/erikh/border/pkg/dnsserver"
	"github.com/erikh/border/pkg/healthcheck"
	"github.com/erikh/border/pkg/lb"
)

type Server struct {
	control       *controlserver.Server
	dns           dnsserver.DNSServer // FIXME make pointer
	balancers     []*lb.Balancer
	healthChecker *healthcheck.HealthChecker
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

				// normalize the listeners to IP:port. Expand the listeners if necessary.
				newListeners := []string{}
				for _, listener := range lbRecord.Listeners {
					host, port, err := net.SplitHostPort(listener)
					if err != nil {
						return nil, fmt.Errorf("Invalid listener %q (is it listed as a peer?): could not parse: %v", listener, err)
					}

					// overwrite the listener peer record with the IP:port internally,
					// this will probably bite me later but is a good solution for now.
					hostIPs := []string{}
					for _, ip := range c.Peers[host].IPs {
						hostIPs = append(hostIPs, ip.String())
						newListeners = append(newListeners, net.JoinHostPort(ip.String(), port))
					}
				}

				lbRecord.Listeners = newListeners

				// second iteration, work with the IP addresses directly.
				for _, listener := range lbRecord.Listeners {
					host, _, err := net.SplitHostPort(listener)
					if err != nil {
						return nil, fmt.Errorf("Invalid listener %q: could not parse: %v", listener, err)
					}

					for _, ip := range c.Peers[peerName].IPs {
						if host == ip.String() {
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
	}

	return balancers, nil
}

func (s *Server) buildHealthChecks(c *config.Config) (*healthcheck.HealthChecker, error) {
	checks := []*healthcheck.HealthCheckAction{}

	for _, zone := range c.Zones {
		for _, rec := range zone.Records {
			switch rec.Type {
			case dnsconfig.TypeA:
				aRecord := rec.Value.(*dnsconfig.A)

				for _, check := range aRecord.HealthCheck {
					for _, ip := range aRecord.Addresses {
						newCheck := check.Copy()
						newCheck.SetTarget(ip.String())

						checks = append(checks, &healthcheck.HealthCheckAction{
							Check: newCheck,
							FailedAction: func(check *healthcheck.HealthCheck) error {
								log.Printf("Health Check for %q (name: %q) failed: pruning A record", newCheck.Target(), newCheck.Name)
								ips := []net.IP{}

								for _, ip := range aRecord.Addresses {
									if ip.String() != check.Target() {
										ips = append(ips, ip)
									}
								}

								aRecord.Addresses = ips
								return nil
							},
							ReviveAction: func(check *healthcheck.HealthCheck) error {
								log.Printf("Health Check for %q (name: %q) revived: adjusting A record", newCheck.Target(), newCheck.Name)

								aRecord.Addresses = append(aRecord.Addresses, net.ParseIP(check.Target()))
								return nil
							},
						})
					}
				}
			case dnsconfig.TypeLB:
			}
		}
	}
	return nil, nil
}
