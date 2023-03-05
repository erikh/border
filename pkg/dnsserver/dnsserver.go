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
	// perform a greedy reverse search of the FQDN. If this code is working
	// right, the longest match will be found first, finding the most local zone.
	// This is probably broken in some subtle way, but YOLO.
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
		// NOTE: according to the docs for Questions, practically, only the first
		// question matters. DNS the specced protocol supports multiple questions,
		// but most servers only honor the first one.
		zone := ds.findZone(r.Question[0])
		switch r.Question[0].Qtype {
		// SOA and NS records are special in our implementation; they always
		// exist just for the zone, and in a specific format. No need for a
		// Match() call here.
		case dns.TypeSOA:
			answers = append(answers, zone.SOA.Convert().(dns.RR))
		case dns.TypeNS:
			for _, rec := range zone.NS.Convert().([]dns.RR) {
				answers = append(answers, rec)
			}
		default:
			for _, rec := range zone.Records {
				if rec.Name == r.Question[0].Name {
					for _, rec := range rec.Value.Convert().([]dns.RR) {
						answers = append(answers, rec)
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
