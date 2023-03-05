package dnsconfig

import (
	"net"

	"github.com/miekg/dns"
)

type Record interface {
	Match(uint16, string) bool
	Convert() any
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

func (soa *SOA) Convert() any {
	return &dns.SOA{
		Ns:      soa.Domain,
		Mbox:    soa.Admin,
		Serial:  soa.Serial,
		Refresh: soa.Refresh,
		Retry:   soa.Retry,
		Expire:  soa.Expire,
		Minttl:  soa.MinTTL,
	}
}

type A struct {
	Name      string   `json:"name"`
	Addresses []net.IP `json:"addresses"`
}

func (a *A) Match(t uint16, s string) bool {
	return t == dns.TypeA && s == a.Name
}

func (a *A) Convert() any {
	ret := []*dns.A{}
	for _, rec := range a.Addresses {
		ret = append(ret, &dns.A{
			A: rec,
		})
	}

	return ret
}

type NS struct {
	Servers []string `json:"servers"`
}

func (ns *NS) Match(t uint16, s string) bool {
	return t == dns.TypeNS
}

func (ns *NS) Convert() any {
	ret := []*dns.NS{}
	for _, rec := range ns.Servers {
		ret = append(ret, &dns.NS{
			Ns: rec,
		})
	}

	return ret
}
