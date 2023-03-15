package dnsserver

import (
	"context"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/dnsconfig"
	"github.com/miekg/dns"
)

func TestStartShutdown(t *testing.T) {
	ds := &DNSServer{
		Zones: map[string]*config.Zone{
			"test.home.arpa.": {
				SOA: &dnsconfig.SOA{
					Domain:  "test.home.arpa.",
					Admin:   "administrator.test.home.arpa.",
					MinTTL:  60,
					Serial:  1,
					Refresh: 60,
					Retry:   60,
					Expire:  60,
				},
				NS: &dnsconfig.NS{
					Servers: []string{"test.home.arpa."},
					TTL:     60,
				},
				Records: []*config.Record{
					{
						Name: "foo.test.home.arpa.",
						Value: &dnsconfig.A{
							Addresses: []net.IP{net.ParseIP("127.0.0.1")},
							TTL:       60,
						},
					},
				},
			},
		},
	}

	if err := ds.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		ds.Shutdown()
	})

	rand.Seed(time.Now().Unix()) // used in msgid calc later

	// because we're using :0 it means that udp and tcp could theoretically be on different ports.
	udpAddr := ds.udpServer.PacketConn.LocalAddr()
	tcpAddr := ds.tcpServer.Listener.Addr()

	udpClient := &dns.Client{Net: "udp"}
	tcpClient := &dns.Client{Net: "tcp"}

	table := map[string]struct {
		addr   net.Addr
		client *dns.Client
	}{
		"udp": {
			addr:   udpAddr,
			client: udpClient,
		},
		"tcp": {
			addr:   tcpAddr,
			client: tcpClient,
		},
	}

	for typ, info := range table {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		conn, err := info.client.DialContext(ctx, info.addr.String())
		if err != nil {
			t.Fatalf("%q DNS server does not appear to start: %v", typ, err)
		}

		id := uint16(rand.Uint32())
		err = conn.WriteMsg(&dns.Msg{MsgHdr: dns.MsgHdr{Id: id, Opcode: dns.OpcodeQuery}, Question: []dns.Question{{Name: "test.home.arpa.", Qtype: dns.TypeSOA, Qclass: dns.ClassINET}}})
		if err != nil {
			t.Fatalf("Error writing message to %q DNS service: %v", typ, err)
		}

		r, err := conn.ReadMsg()
		if err != nil {
			t.Fatalf("Error reading message to %q DNS service: %v", typ, err)
		}

		if r.Id != id {
			t.Fatalf("Unexpected message ID %d (expected: %d) returned in %q DNS service: %v", r.Id, id, typ, err)
		}

		if r.Rcode != dns.RcodeSuccess {
			t.Fatalf("Unexpected message returned in %q DNS service: %v", typ, err)
		}

		if len(r.Answer) != 1 {
			t.Fatalf("Invalid number of answers in query to %q DNS service", typ)
		}

		soa, ok := r.Answer[0].(*dns.SOA)
		if !ok {
			t.Fatalf("Answer was not SOA record in response for %q service", typ)
		}

		if soa.Ns != "test.home.arpa." {
			t.Fatalf("Unexpected result in SOA record for %q service: %q", typ, soa.Ns)
		}

		id = uint16(rand.Uint32())
		err = conn.WriteMsg(&dns.Msg{MsgHdr: dns.MsgHdr{Id: id, Opcode: dns.OpcodeQuery}, Question: []dns.Question{{Name: "test.home.arpa.", Qtype: dns.TypeNS, Qclass: dns.ClassINET}}})
		if err != nil {
			t.Fatalf("Error writing message to %q DNS service: %v", typ, err)
		}

		r, err = conn.ReadMsg()
		if err != nil {
			t.Fatalf("Error reading message to %q DNS service: %v", typ, err)
		}

		if r.Id != id {
			t.Fatalf("Unexpected message ID %d (expected: %d) returned in %q DNS service: %v", r.Id, id, typ, err)
		}

		if r.Rcode != dns.RcodeSuccess {
			t.Fatalf("Unexpected message returned in %q DNS service: %v", typ, err)
		}

		if len(r.Answer) != 1 {
			t.Fatalf("Invalid number of Ns answers in query to %q DNS service", typ)
		}

		ns, ok := r.Answer[0].(*dns.NS)
		if !ok {
			t.Fatalf("Answer was not NS record in response for %q service", typ)
		}

		if ns.Ns != "test.home.arpa." {
			t.Fatalf("Unexpected result in SOA record for %q service: %q", typ, ns.Ns)
		}

		id = uint16(rand.Uint32())
		err = conn.WriteMsg(&dns.Msg{MsgHdr: dns.MsgHdr{Id: id, Opcode: dns.OpcodeQuery}, Question: []dns.Question{{Name: "foo.test.home.arpa.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}})
		if err != nil {
			t.Fatalf("Error writing message to %q DNS service: %v", typ, err)
		}

		r, err = conn.ReadMsg()
		if err != nil {
			t.Fatalf("Error reading message to %q DNS service: %v", typ, err)
		}

		if r.Id != id {
			t.Fatalf("Unexpected message ID %d (expected: %d) returned in %q DNS service: %v", r.Id, id, typ, err)
		}

		if r.Rcode != dns.RcodeSuccess {
			t.Fatalf("Unexpected message returned in %q DNS service: %v", typ, err)
		}

		if len(r.Answer) != 1 {
			t.Fatalf("Invalid number of Ns answers in query to %q DNS service", typ)
		}

		a, ok := r.Answer[0].(*dns.A)
		if !ok {
			t.Fatalf("Answer was not A record in response for %q service", typ)
		}

		if a.A.String() != "127.0.0.1" {
			t.Fatalf("Unexpected result in SOA record for %q service: %q", typ, a.A)
		}

		conn.Close()
	}
}

func TestFindZone(t *testing.T) {
	ds := &DNSServer{
		Zones: map[string]*config.Zone{
			"test.home.arpa.": {
				SOA: &dnsconfig.SOA{
					Domain: "test.home.arpa.",
				},
			},
			"home.arpa.": {
				SOA: &dnsconfig.SOA{
					Domain: "home.arpa.",
				},
			},
			"zombo.com.": {
				SOA: &dnsconfig.SOA{
					Domain: "zombo.com.",
				},
			},
		},
	}

	zone := ds.findZone("facebook.com.")
	if zone != nil {
		t.Fatal("invalid zone facebook.com was matched")
	}

	for zone := range ds.Zones {
		selectedZone := ds.findZone(zone)
		if selectedZone == nil || selectedZone.SOA.Domain != zone {
			t.Fatalf("domain %q did not match", zone)
		}
	}
}
