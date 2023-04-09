package acmekit

import (
	"context"
	"net/url"
	"testing"

	"github.com/erikh/duct"
)

func createPebble(t *testing.T) {
	d := duct.New(duct.Manifest{
		{
			Name:  "pebble",
			Image: "letsencrypt/pebble:latest",
			PortForwards: map[int]int{
				14000: 14000,
				15000: 15000,
			},
			Command: []string{"/bin/sh", "-c", "pebble -config /test/config/pebble-config.json -strict -dnsserver 10.30.50.3:8053"},
			IPv4:    "10.30.50.4",
		},
		{
			Name:  "pebble-challtestsrv",
			Image: "letsencrypt/pebble-challtestsrv:latest",
			PortForwards: map[int]int{
				8055: 8055,
			},
			Command: []string{"/bin/sh", "-c", `pebble-challtestsrv -defaultIPv6 "" -defaultIPv4 10.30.50.3`},
			IPv4:    "10.30.50.3",
		},
	}, duct.WithNewNetworkAndSubnet("acmekit-test", "10.30.50.0/24"))

	d.HandleSignals(true)

	t.Cleanup(func() {
		if err := d.Teardown(context.Background()); err != nil {
			t.Fatal(err)
		}
	})

	if err := d.Launch(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestACMEAccount(t *testing.T) {
	createPebble(t)

	directory, err := url.Parse("https://127.0.0.1:14000/dir")
	if err != nil {
		t.Fatal(err)
	}

	ap := &ACMEParams{
		IgnoreVerify: true,
		Directory:    directory,
		ContactInfo:  []string{"mailto:erik@hollensbe.org"},
	}

	if err := ap.CreateAccount(context.Background()); err != nil {
		t.Fatal(err)
	}

	valid, err := ap.HasValidAccount(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if !valid {
		t.Fatal("ACME account wasn't valid")
	}
}
