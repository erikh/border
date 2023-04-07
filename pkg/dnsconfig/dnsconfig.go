package dnsconfig

import (
	"net"
	"time"

	"github.com/erikh/border/pkg/healthcheck"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
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
	Domain  string `record:"domain"`
	Admin   string `record:"admin"`
	MinTTL  uint32 `record:"minttl"`
	Serial  uint32 `record:"serial"`
	Refresh uint32 `record:"refresh"`
	Retry   uint32 `record:"retry"`
	Expire  uint32 `record:"expire"`
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
	Addresses   []net.IP                   `record:"addresses"`
	TTL         uint32                     `record:"ttl,optional"`
	HealthCheck []*healthcheck.HealthCheck `record:"healthcheck,optional"`
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
	Servers []string `record:"servers"`
	TTL     uint32   `record:"ttl,optional"`
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

type TLSLB struct {
	CACertificate []byte `record:"ca_certificate,optional"`
	Certificate   []byte `record:"certificate"`
	Key           []byte `record:"key"`
}

type LB struct {
	Listeners                []string                   `record:"listeners"`
	Kind                     string                     `record:"kind"`
	Backends                 []string                   `record:"backends"`
	SimultaneousConnections  int                        `record:"simultaneous_connections,optional"`
	MaxConnectionsPerAddress int                        `record:"max_connections_per_address,optional"`
	ConnectionTimeout        time.Duration              `record:"connection_timeout,optional"`
	TTL                      uint32                     `record:"ttl,optional"`
	TLS                      *TLSLB                     `record:"tls,optional"`
	HealthCheck              []*healthcheck.HealthCheck `record:"healthcheck,optional"`
}

func (lb *LB) Convert(name string) []dns.RR {
	ret := []dns.RR{}
	for _, listener := range lb.Listeners {
		host, _, err := net.SplitHostPort(listener)
		if err != nil {
			logrus.Fatalf("Conversion error in listener %q converting to Peer IP: %v", listener, err)
			return nil
		}

		ip := net.ParseIP(host)

		if ip == nil {
			logrus.Fatalf("Invalid IP after Peer conversion (%q) for listener %q", host, listener)
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
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    lb.TTL,
				},
				AAAA: ip,
			}))
		}
	}

	return ret
}
