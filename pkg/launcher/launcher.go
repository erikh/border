package launcher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/erikh/border/pkg/acmekit"
	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/controlclient"
	"github.com/erikh/border/pkg/controlserver"
	"github.com/erikh/border/pkg/dnsconfig"
	"github.com/erikh/border/pkg/dnsserver"
	"github.com/erikh/border/pkg/election"
	"github.com/erikh/border/pkg/healthcheck"
	"github.com/erikh/border/pkg/lb"
	"github.com/erikh/go-hashchain"
	"github.com/mholt/acmez"
	"github.com/mholt/acmez/acme"
	"github.com/sirupsen/logrus"
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
	if os.Getenv("DEBUG_LOG") != "" {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	peer, err := c.FindPeer(peerName)
	if err != nil {
		return fmt.Errorf("Could not find the name of this peer: %q: %w", peerName, err)
	}

	cs, err := controlserver.Start(c, peer, c.Listen.Control, controlserver.NonceExpiration, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("Error while starting control server: %w", err)
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
	go s.monitorConfig()

	// this should be the last thing that runs!
	if err := s.holdElection(); err != nil {
		return fmt.Errorf("Error while holding election: %w", err)
	}

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

func (s *Server) monitorConfig() {
	for {
		time.Sleep(time.Second)
		publisher := s.config.GetPublisher()

		if publisher == nil {
			logrus.Infoln("Publisher was unset; waiting for publisher to appear")
			continue
		}

		if publisher.Name() == s.peerName {
			continue
		}

		client := controlclient.FromPeer(publisher)
		resp, err := client.Exchange(&api.ConfigChainRequest{}, true)
		if err != nil {
			logrus.Errorf("Could not get configuration chain from publisher %q: %v", publisher.Name(), err)
			continue
		}

		publisherChain, err := hashchain.NewFromString(resp.(*api.ConfigChainResponse).Chain)
		if err != nil {
			logrus.Errorf("Chain from publisher %q was invalid: %v", publisher.Name(), err)
			continue
		}

		chainSum, err := s.config.Chain().Sum(config.HashFunc())
		if err != nil {
			logrus.Errorf("Could not sum config chain: %v", err)
			continue
		}

		publisherSum, err := publisherChain.Sum(config.HashFunc())
		if err != nil {
			logrus.Errorf("Could not sum publisher %q config chain: %v", publisher.Name(), err)
			continue
		}

		if chainSum != publisherSum {
			// ensure our chain is an ancestor of the publisher
			if _, err := publisherChain.LastMatch(s.config.Chain()); err != nil {
				logrus.Errorf("Publisher %q's configuration never had our configuration as an ancestor: %v", publisher.Name(), err)
				continue // FIXME not sure what to do here really
			}

			resp, err := client.Exchange(&api.ConfigFetchRequest{}, true)
			if err != nil {
				logrus.Errorf("Config changed in publisher %q, but could not fetch updated configuration: %v", publisher.Name(), err)
				continue
			}

			newConfigResp := resp.(*api.ConfigFetchResponse)
			newChain, err := hashchain.NewFromString(newConfigResp.Chain)
			if err != nil {
				logrus.Errorf("Could not parse generational chain of configuration from publisher %q: %v", publisher.Name(), err)
				continue
			}

			if err := s.control.ReplaceConfig(newConfigResp.Config, newChain); err != nil {
				logrus.Errorf("Could not persist configuration from publisher %q locally: %v", publisher.Name(), err)
				continue
			}
		}
	}
}

func (s *Server) monitorReload() {
retry:
	select {
	case <-time.After(time.Second):
		goto retry
	case <-s.config.ReloadChan():
	}

	// FIXME probably should make this timeout configurable
	// FIXME need graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	if err := s.Shutdown(ctx); err != nil {
		logrus.Errorf("Error while shutting down for config reload: %v", err)
	}
	cancel() // no-op, but needed to clean up

	logrus.Infoln("New configuration received; reloading services")

	s2 := &Server{}

	if err := s2.Launch(s.peerName, s.config); err != nil {
		logrus.Errorf("Error launching server after reload: %v", err)
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

				var tls *lb.TLSBalancerConfig

				if lbRecord.ACME != nil {
					var solver acmez.Solver

					switch lbRecord.ACME.ChallengeType {
					case acme.ChallengeTypeDNS01:
						solver = c.DNSSolver(rec.Name)
					case acme.ChallengeTypeTLSALPN01:
						solver = c.ALPNSolver(rec.Name)
					case acme.ChallengeTypeHTTP01:
						solver = c.HTTPSolver(rec.Name)
					default:
						return nil, fmt.Errorf("%q is not a valid ACME challenge type", lbRecord.ACME.ChallengeType)
					}

					cert, err := c.ACME.GetCertificate(context.Background(), rec.Name, acmekit.Solvers{lbRecord.ACME.ChallengeType: solver})
					if err != nil {
						return nil, fmt.Errorf("Error obtaining ACME certificate: %w", err)
					}

					tls = &lb.TLSBalancerConfig{
						Certificate: cert.Chain,
						Key:         cert.PrivateKey,
					}
				} else if lbRecord.TLS != nil {
					tls = &lb.TLSBalancerConfig{
						CACertificate: lbRecord.TLS.CACertificate,
						Certificate:   lbRecord.TLS.Certificate,
						Key:           lbRecord.TLS.Key,
					}
				}

				// work with the IP addresses directly.
				for _, listener := range lbRecord.Listeners {
					host, port, err := net.SplitHostPort(listener)
					if err != nil {
						return nil, fmt.Errorf("Invalid listener %q: could not parse: %v", listener, err)
					}

					peer, err := c.FindPeer(host)
					if err != nil {
						return nil, fmt.Errorf("Host %q is not a peer: %w", host, err)
					}

					for _, ip := range peer.IPs {
						bc := lb.BalancerConfig{
							Kind:                     lbRecord.Kind,
							Backends:                 lbRecord.Backends,
							SimultaneousConnections:  lbRecord.SimultaneousConnections,
							MaxConnectionsPerAddress: lbRecord.MaxConnectionsPerAddress,
							ConnectionTimeout:        lbRecord.ConnectionTimeout,
							TLS:                      tls,
						}

						balancer := lb.Init(net.JoinHostPort(ip.String(), port), bc)
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

func (s *Server) holdElection() error {
	e := election.NewElection(s.config)
	peer, err := e.Vote()
	if err != nil {
		return err
	}

	logrus.Infof("Electing %q as new leader", peer.Name())
	s.config.SetPublisher(peer)
	return nil
}

func (s *Server) buildHealthChecks(c *config.Config) (*healthcheck.HealthChecker, error) {
	checks := []*healthcheck.HealthCheckAction{}

	for _, peer := range c.Peers {
		check := &healthcheck.HealthCheck{
			Name: fmt.Sprintf("%q peer check", peer.Name()),
			Type: healthcheck.TypeHTTP,
			// FIXME these shouldn't be hard coded. Maybe keep them as a part of the
			// peer
			Timeout:  time.Second,
			Failures: 3,
		}

		check.SetTarget(fmt.Sprintf("http://%s/%s", peer.ControlServer, api.PathPing))
		client := controlclient.FromPeer(peer)
		innerPeer := peer // golang, for loops shouldn't work that way

		check.SetRequestTransformer(func(req interface{}) interface{} {
			r := req.(*http.Request)
			out, err := client.PrepareRequest(&api.PingRequest{}, true)
			if err != nil {
				logrus.Errorf("While preparing HTTP ping monitor for peer %q: %v", innerPeer.Name(), err)
			}
			r.Method = http.MethodPut
			r.Body = io.NopCloser(bytes.NewBuffer(out))
			return r
		})

		revived := func(hc *healthcheck.HealthCheck) error {
			s.config.AddPeer(innerPeer)
			return s.holdElection()
		}

		failed := func(hc *healthcheck.HealthCheck) error {
			s.config.RemovePeer(innerPeer)
			return s.holdElection()
		}

		checks = append(checks, &healthcheck.HealthCheckAction{
			Check:        check,
			FailedAction: failed,
			ReviveAction: revived,
		})
	}

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
								logrus.Errorf("Health Check for %q (name: %q) failed: pruning A record", newCheck.Target(), newCheck.Name)
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
								logrus.Infof("Health Check for %q (name: %q) revived: adjusting A record", newCheck.Target(), newCheck.Name)

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
								logrus.Errorf("Health Check for %q (name: %q) failed: pruning LB backend record", newCheck.Target(), newCheck.Name)
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
								logrus.Infof("Health Check for %q (name: %q) revived: adjusting LB record", newCheck.Target(), newCheck.Name)

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

						peer, err := s.config.FindPeer(host)
						if err != nil {
							return nil, fmt.Errorf("Could not locate IPs for peer %q: %w", peer.Name(), err)
						}

						for _, ip := range peer.IPs {
							newCheck := check.Copy()

							if newCheck.Name == "" {
								newCheck.Name = rec.Name
							}

							if newCheck.Type == "" {
								newCheck.Type = healthcheck.TypePing
							}

							newCheck.SetTarget(ip.String())

							checks = append(checks, &healthcheck.HealthCheckAction{
								Check: newCheck,
								FailedAction: func(check *healthcheck.HealthCheck) error {
									logrus.Errorf("Health Check for %q (name: %q) failed: pruning LB backend record", newCheck.Target(), newCheck.Name)
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
									logrus.Infof("Health Check for %q (name: %q) revived: adjusting LB record", newCheck.Target(), newCheck.Name)

									lbRecord.Listeners = append(lbRecord.Listeners, listener)
									return nil
								},
							})
						}
					}
				}
			}
		}
	}

	return healthcheck.Init(checks, time.Second), nil
}
