package config

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/erikh/border/pkg/dnsconfig"
	"github.com/erikh/border/pkg/josekit"
)

func TestRecordParse(t *testing.T) {
	key, err := josekit.MakeKey("test")
	if err != nil {
		t.Fatal(err)
	}

	config := Config{
		Listen: ListenConfig{Control: ":5309"},
		Peers: []*Peer{{
			Key:           key,
			IPs:           []net.IP{net.ParseIP("127.0.0.1")},
			ControlServer: "127.0.0.1:5309",
		}},
		Zones: map[string]*Zone{
			"test.home.arpa": {
				SOA: &dnsconfig.SOA{
					Domain:  "test.home.arpa",
					Admin:   "administrator.test.home.arpa",
					MinTTL:  60,
					Serial:  1,
					Refresh: 60,
					Retry:   1,
					Expire:  120,
				},
				NS: &dnsconfig.NS{
					Servers: []string{"test.home.arpa"},
				},
				Records: []*Record{
					{
						Type: dnsconfig.TypeA,
						Name: "foo.test.home.arpa",
						LiteralValue: map[string]any{
							"addresses": []string{
								"127.0.0.1",
							},
						},
					},
					{
						Type: dnsconfig.TypeLB,
						Name: "border.test.home.arpa",
						LiteralValue: map[string]any{
							"listeners": []string{"test"},
							"kind":      "tcp",
							"backends": []string{
								"127.0.0.1:80",
							},
							"simultaneous_connections":    100,
							"max_connections_per_address": 1000,
							"connection_timeout":          10 * time.Millisecond, // YAML/JSON converts this to int64
						},
					},
				},
			},
		},
	}

	if err := config.convertLiterals(); err != nil {
		t.Fatal(err)
	}

	aRecord := config.Zones["test.home.arpa"].Records[0].Value.(*dnsconfig.A)
	realARecord := &dnsconfig.A{
		Addresses: []net.IP{net.ParseIP("127.0.0.1")},
		TTL:       60,
	}

	if !reflect.DeepEqual(realARecord, aRecord) {
		t.Fatal("A records did not match")
	}

	lbRecord := config.Zones["test.home.arpa"].Records[1].Value.(*dnsconfig.LB)
	realLBRecord := &dnsconfig.LB{
		Listeners:                []string{"test"},
		Kind:                     "tcp",
		Backends:                 []string{"127.0.0.1:80"},
		SimultaneousConnections:  100,
		MaxConnectionsPerAddress: 1000,
		ConnectionTimeout:        10 * time.Millisecond,
		TTL:                      60,
	}

	if !reflect.DeepEqual(realLBRecord, lbRecord) {
		t.Log(lbRecord)
		t.Fatal("LB records did not match")
	}

}
