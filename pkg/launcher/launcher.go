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
	config        *config.Config
	peerName      string
}

func (s *Server) Launch(peerName string, c *config.Config) error {
	peer, err := c.FindPeer(peerName)
	if err != nil {
		return err
	}

	cs, err := controlserver.Start(c, peer, c.Listen.Control, controlserver.NonceExpiration, 100*time.Millisecond)
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

	s.peerName = peerName
	s.config = c
	s.dns = dnsserver
	s.control = cs
	s.balancers = balancers

	healthchecker, err := s.buildHealthChecks(c)
	if err != nil {
		return fmt.Errorf("While constructing health check subsystem: %w", err)
	}

	s.healthChecker = healthchecker
	s.healthChecker.Start()

	go s.monitorReload()

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.healthChecker.Shutdown()

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

func (s *Server) monitorReload() {
	for {
		// we can't just wait on the channel, because it's replaced when the
		// configuration is reloaded, and so it will be different by _the time_ the
		// configuration would signal this channel.
		select {
		case <-s.config.ReloadChan():
		default:
			time.Sleep(time.Second)
			continue
		}

		// FIXME probably should make this timeout configurable
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		if err := s.Shutdown(ctx); err != nil {
			log.Printf("Error while shutting down for config reload: %v", err)
		}
		cancel() // no-op, but needed to clean up

		if err := s.Launch(s.peerName, s.config); err != nil {
			log.Fatalf("Error launching server after reload: %v", err)
		}

		return // a new routine will be launched from Launch(), so cancel this one
	}
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

					peer, err := c.FindPeer(host)
					if err != nil {
						return nil, fmt.Errorf("Host %q is not a peer: %w", host, err)
					}

					// overwrite the listener peer record with the IP:port internally,
					// this will probably bite me later but is a good solution for now.
					for _, ip := range peer.IPs {
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

					peer, err := c.FindPeer(host)
					if err != nil {
						return nil, fmt.Errorf("Host %q is not a peer: %w", host, err)
					}

					for _, ip := range peer.IPs {
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

						if newCheck.Name == "" {
							newCheck.Name = rec.Name
						}

						if newCheck.Type == "" {
							newCheck.Type = healthcheck.TypePing
						}

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
				lbRecord := rec.Value.(*dnsconfig.LB)

				for _, check := range lbRecord.HealthCheck {
					for _, backend := range lbRecord.Backends {
						newCheck := check.Copy()

						host, _, err := net.SplitHostPort(backend)
						if err != nil {
							return nil, fmt.Errorf("While computing healthcheck records for load balancer backend %q: %w", backend, err)
						}

						if newCheck.Name == "" {
							newCheck.Name = rec.Name
						}

						if newCheck.Type == "" {
							newCheck.Type = healthcheck.TypePing
						}

						newCheck.SetTarget(host)

						checks = append(checks, &healthcheck.HealthCheckAction{
							Check: newCheck,
							FailedAction: func(check *healthcheck.HealthCheck) error {
								log.Printf("Health Check for %q (name: %q) failed: pruning LB backend record", newCheck.Target(), newCheck.Name)
								backends := []string{}

								for _, be := range lbRecord.Backends {
									if be != backend {
										backends = append(backends, be)
									}
								}

								lbRecord.Backends = backends
								return nil
							},
							ReviveAction: func(check *healthcheck.HealthCheck) error {
								log.Printf("Health Check for %q (name: %q) revived: adjusting LB record", newCheck.Target(), newCheck.Name)

								lbRecord.Backends = append(lbRecord.Backends, backend)
								return nil
							},
						})
					}

					for _, listener := range lbRecord.Listeners {
						host, _, err := net.SplitHostPort(listener)
						if err != nil {
							return nil, fmt.Errorf("While computing healthcheck records for load balancer listener %q: %w", listener, err)
						}

						newCheck := check.Copy()

						if newCheck.Name == "" {
							newCheck.Name = rec.Name
						}

						if newCheck.Type == "" {
							newCheck.Type = healthcheck.TypePing
						}

						newCheck.SetTarget(host)

						checks = append(checks, &healthcheck.HealthCheckAction{
							Check: newCheck,
							FailedAction: func(check *healthcheck.HealthCheck) error {
								log.Printf("Health Check for %q (name: %q) failed: pruning LB backend record", newCheck.Target(), newCheck.Name)
								listeners := []string{}

								for _, lis := range lbRecord.Listeners {
									if lis != listener {
										listeners = append(listeners, lis)
									}
								}

								lbRecord.Listeners = listeners
								return nil
							},
							ReviveAction: func(check *healthcheck.HealthCheck) error {
								log.Printf("Health Check for %q (name: %q) revived: adjusting LB record", newCheck.Target(), newCheck.Name)

								lbRecord.Listeners = append(lbRecord.Listeners, listener)
								return nil
							},
						})
					}
				}
			}
		}
	}

	return healthcheck.Init(checks, time.Second), nil
}
