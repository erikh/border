package dnsconfig

import (
	"net"

	"github.com/miekg/dns"
)

// An attempt to normalize record management so it can be addressed in a
// generic way in the server. Functionally, you use Match() with the record
// type and name for a linear search of all records, but this is duplicated in
// the config object, which is less overhead. Convert() turns it into a []dns.RR
// in almost all cases, but SOA records are special as they are a singleton.
//
// FIXME I think Match will be useful for additional situations, but right now it's
// kind of dead weight.
type Record interface {
	Match(uint16, string) bool
	Convert() []dns.RR
}

type SOA struct {
	Domain  string `json:"domain"`
	Admin   string `json:"admin"`
	MinTTL  uint32 `json:"minttl"`
	Serial  uint32 `json:"serial"`
	Refresh uint32 `json:"refresh"`
	Retry   uint32 `json:"retry"`
	Expire  uint32 `json:"expire"`
}

func (soa *SOA) Match(t uint16, s string) bool {
	return t == dns.TypeSOA && s == soa.Domain
}

func (soa *SOA) Convert() []dns.RR {
	return []dns.RR{&dns.SOA{
		Hdr: dns.RR_Header{
			Name:   soa.Domain,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    soa.MinTTL,
		},
		Ns:      soa.Domain,
		Mbox:    soa.Admin,
		Serial:  soa.Serial,
		Refresh: soa.Refresh,
		Retry:   soa.Retry,
		Expire:  soa.Expire,
		Minttl:  soa.MinTTL,
	}}
}

type A struct {
	Name      string   `json:"name"`
	Addresses []net.IP `json:"addresses"`
	TTL       uint32
}

func (a *A) Match(t uint16, s string) bool {
	return t == dns.TypeA && s == a.Name
}

func (a *A) Convert() []dns.RR {
	ret := []dns.RR{}
	for _, rec := range a.Addresses {
		ret = append(ret, dns.RR(&dns.A{
			Hdr: dns.RR_Header{
				Name:   a.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    a.TTL,
			},
			A: rec,
		}))
	}

	return ret
}

type NS struct {
	Servers []string `json:"servers"`
	TTL     uint32
}

func (ns *NS) Match(t uint16, s string) bool {
	return t == dns.TypeNS
}

func (ns *NS) Convert() []dns.RR {
	ret := []dns.RR{}
	for _, rec := range ns.Servers {
		ret = append(ret, dns.RR(&dns.NS{
			Hdr: dns.RR_Header{
				Name:   rec,
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    ns.TTL,
			},
			Ns: rec,
		}))
	}

	return ret
}
