package dnsserver

import (
	"testing"

	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/dnsconfig"
)

func TestFindZone(t *testing.T) {
	ds := &DNSServer{
		Zones: map[string]config.Zone{
			"test.home.arpa": {
				SOA: dnsconfig.SOA{
					Domain: "test.home.arpa",
				},
			},
			"home.arpa": {
				SOA: dnsconfig.SOA{
					Domain: "home.arpa",
				},
			},
			"zombo.com": {
				SOA: dnsconfig.SOA{
					Domain: "zombo.com",
				},
			},
		},
	}

	zone := ds.findZone("facebook.com")
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
