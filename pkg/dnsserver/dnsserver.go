package dnsserver

import (
	"errors"
	"strings"

	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/dnsconfig"

	"github.com/miekg/dns"
)

type DNSServer struct {
	Zones     map[string]*config.Zone
	udpServer *dns.Server
	tcpServer *dns.Server
}

// Start returns after the servers have started, and launches a UDP and TCP
// server in the background on the network specification.
func (ds *DNSServer) Start(listenSpec string) error {
	// the only reason this is 4 is because if the goroutine listens terminate
	// prematurely after starting, they may yield an error, which would deadlock
	// the channel, so fill the buffer pointlessly, but at least nothing locks
	// up. Shutdown() will catch the real error. There's probably a good argument
	// for just returning the channel here instead.
	done := make(chan error, 4)
	startFunc := func() {
		done <- nil
	}

	ds.udpServer = &dns.Server{Addr: listenSpec, Net: "udp", Handler: ds, NotifyStartedFunc: startFunc, ReusePort: true}
	ds.tcpServer = &dns.Server{Addr: listenSpec, Net: "tcp", Handler: ds, NotifyStartedFunc: startFunc}

	go func() {
		switch err := ds.udpServer.ListenAndServe(); err {
		case nil:
		default:
			done <- err
		}
	}()

	go func() {
		switch err := ds.tcpServer.ListenAndServe(); err {
		case nil:
		default:
			done <- err
		}
	}()

	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			return err
		}
	}

	return nil
}

// Shutdown the server. Returns errors on unstarted servers or failed
// shutdowns.
func (ds *DNSServer) Shutdown() error {
	if ds.udpServer == nil || ds.tcpServer == nil {
		return errors.New("cannot shutdown server; never started")
	}

	if err := ds.udpServer.Shutdown(); err != nil {
		return errors.Join(err, errors.New("unable to shutdown UDP server"))
	}

	if err := ds.tcpServer.Shutdown(); err != nil {
		return errors.Join(err, errors.New("unable to shutdown TCP server"))
	}

	return nil
}

func (ds *DNSServer) findZone(name string) *config.Zone {
	names := dns.SplitDomainName(name)
	// perform a greedy reverse search of the FQDN. If this code is working
	// right, the longest match will be found first, finding the most local zone.
	for i := len(names); i > 0; i-- {
		potentialZone := strings.Join(names[len(names)-i:], ".") + "."
		if zone, ok := ds.Zones[potentialZone]; ok {
			return zone
		}
	}

	return nil
}

func (ds *DNSServer) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := &dns.Msg{}
	m.SetReply(r)

	answers := []dns.RR{}

	if len(r.Question) != 0 {
		// NOTE: according to the docs for Questions, practically, only the first
		// question matters. DNS the specced protocol supports multiple questions,
		// but most servers only honor the first one. So we are going to avoid
		// caring about any others and save ourselves some trouble.
		name := r.Question[0].Name
		zone := ds.findZone(name)

		if zone != nil {
			typ := r.Question[0].Qtype
			switch typ {
			// SOA and NS are special because they are special records.
			case dns.TypeSOA:
				answers = zone.SOA.Convert(name)
			case dns.TypeNS:
				answers = zone.NS.Convert(name)
			default:
				for _, rec := range zone.Records {
					if rec.Name == name {
						switch rec.Type {
						case dnsconfig.TypeA:
							// we don't want to deliver answers for non-A queries for these records.
							if typ == dns.TypeA {
								answers = rec.Value.Convert(name)
							}
						case dnsconfig.TypeLB:
							values := rec.Value.Convert(name)

							for _, answer := range values {
								switch a := answer.(type) {
								// filter the right records for the query type
								case *dns.A:
									if a.Hdr.Rrtype == typ {
										answers = append(answers, answer)
									}
								case *dns.AAAA:
									if a.Hdr.Rrtype == typ {
										answers = append(answers, answer)
									}
								}
							}
						}

						break
					}
				}
			}
		}
	}

	if len(answers) == 0 {
		m.SetRcode(r, dns.RcodeNameError)
		w.WriteMsg(m) // nolint:errcheck
		return
	}

	m.Authoritative = true
	// NOTE we should verify if this setting is absolutely necessary, I cargo culted it from some old code.
	m.RecursionAvailable = true
	m.Answer = answers

	w.WriteMsg(m) // nolint:errcheck
}
