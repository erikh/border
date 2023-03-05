package dnsserver

import (
	"strings"
	"sync"

	"github.com/erikh/border/pkg/config"
	"github.com/miekg/dns"
)

type DNSServer struct {
	Zones        map[string]config.Zone
	rebuildMutex sync.RWMutex
}

func (ds *DNSServer) Listen(listenSpec string) error {
	return dns.ListenAndServe(listenSpec, "udp", ds)
}

func (ds *DNSServer) findZone(question dns.Question) *config.Zone {
	names := dns.SplitDomainName(question.Name)
	for i := len(names); i >= 1; i-- {
		potentialZone := strings.Join(names[len(names)-i-1:len(names)-1], ".")
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
		// NOTE: according to the docs for Questions, practically, only the first question matters
		zone := ds.findZone(r.Question[0])
		switch r.Question[0].Qtype {
		case dns.TypeSOA:
			answers = append(answers, zone.SOA.Convert().(dns.RR))
		case dns.TypeNS:
			for _, rec := range zone.NS.Convert().([]dns.RR) {
				answers = append(answers, rec)
			}
		default:
			// for _, rec := range zone.Records {
			// }
		}
	}

	if len(answers) == 0 {
		m.SetRcode(r, dns.RcodeSuccess)
		w.WriteMsg(m)
		return
	}

	m.Authoritative = true
	m.RecursionAvailable = true
	m.Answer = answers

	w.WriteMsg(m)
}
