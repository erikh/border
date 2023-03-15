package dnsconfig

import (
	"log"
	"net"

	"github.com/miekg/dns"
)

const (
	TypeA  = "A"
	TypeLB = "LB"
)

// An attempt to normalize record management so it can be addressed in a
// generic way in the server. Convert() turns it into a []dns.RR in all
// cases, required for different record types to be implemented manually.
//
// Honestly it kind of sucks, but no generics soooooo...
type Record interface {
	Convert(string) []dns.RR
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

const (
	DefaultSimultaneousConnections  = 16384
	DefaultMaxConnectionsPerAddress = 32768
)

type LB struct {
	Listeners                []string `json:"listeners"`
	Kind                     string   `json:"kind"`
	Backends                 []string `json:"backends"`
	SimultaneousConnections  uint     `json:"simultaneous_connections"`
	MaxConnectionsPerAddress uint64   `json:"max_connections_per_address"`
	TTL                      uint32   `json:"ttl"`
}

func (lb *LB) Convert(name string) []dns.RR {
	ret := []dns.RR{}
	for _, listener := range lb.Listeners {
		host, _, err := net.SplitHostPort(listener)
		if err != nil {
			log.Fatalf("Conversion error in listener %q converting to Peer IP: %v", listener, err)
			return nil
		}

		ip := net.ParseIP(host)

		if ip == nil {
			log.Fatalf("Invalid IP after Peer conversion (%q) for listener %q", host, listener)
			return nil
		} else if ip.To4() != nil {
			ret = append(ret, dns.RR(&dns.A{
				Hdr: dns.RR_Header{
					Name:   name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    lb.TTL,
				},
				A: ip,
			}))
		} else {
			ret = append(ret, dns.RR(&dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    lb.TTL,
				},
				AAAA: ip,
			}))
		}
	}

	return ret
}
