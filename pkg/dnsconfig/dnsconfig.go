package dnsconfig

import (
	"net"

	"github.com/miekg/dns"
)

const (
	TypeA = "A"
)

// An attempt to normalize record management so it can be addressed in a
// generic way in the server. Convert() turns it into a []dns.RR in all
// cases, required for different record types to be implemented manually.
//
// Honestly it kind of sucks, but no generics soooooo...
type Record interface {
	Convert(name string) []dns.RR
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

func (soa *SOA) Convert(name string) []dns.RR {
	return []dns.RR{&dns.SOA{
		Hdr: dns.RR_Header{
			Name:   name,
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
	Addresses []net.IP `json:"addresses"`
	TTL       uint32   `json:"ttl"`
}

func (a *A) Convert(name string) []dns.RR {
	ret := []dns.RR{}
	for _, rec := range a.Addresses {
		ret = append(ret, dns.RR(&dns.A{
			Hdr: dns.RR_Header{
				Name:   name,
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
	TTL     uint32   `json:"ttl"`
}

func (ns *NS) Convert(name string) []dns.RR {
	ret := []dns.RR{}
	for _, rec := range ns.Servers {
		ret = append(ret, dns.RR(&dns.NS{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    ns.TTL,
			},
			Ns: rec,
		}))
	}

	return ret
}
