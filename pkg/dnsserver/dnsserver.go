package dnsserver

import (
	"errors"
	"strings"

	"github.com/erikh/border/pkg/config"
	"github.com/miekg/dns"
)

type DNSServer struct {
	Zones     map[string]config.Zone
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
		potentialZone := strings.Join(names[len(names)-i:len(names)], ".") + "."
		if zone, ok := ds.Zones[potentialZone]; ok {
			return &zone
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
		zone := ds.findZone(r.Question[0].Name)

		if zone != nil {
			switch r.Question[0].Qtype {
			// SOA and NS are special because they are special records.
			case dns.TypeSOA:
				answers = zone.SOA.Convert()
			case dns.TypeNS:
				for _, ns := range zone.NS.Convert() {
					answers = append(answers, ns)
				}
			default:
				for _, rec := range zone.Records {
					if rec.Name == r.Question[0].Name {
						for _, rec := range rec.Value.Convert() {
							answers = append(answers, rec)
						}
					}
				}
			}
		}
	}

	if len(answers) == 0 {
		m.SetRcode(r, dns.RcodeSuccess)
		w.WriteMsg(m)
		return
	}

	m.Authoritative = true
	// NOTE we should verify if this setting is absolutely necessary, I cargo culted it from some old code.
	m.RecursionAvailable = true
	m.Answer = answers

	w.WriteMsg(m)
}
