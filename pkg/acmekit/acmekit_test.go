package acmekit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/dnsconfig"
	"github.com/erikh/border/pkg/dnsserver"
	"github.com/erikh/duct"
	"github.com/mholt/acmez/acme"
)

const (
	Domain  = "example.org"
	DNSPort = "5300"
)

func getExternalIP(t *testing.T) string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}

	// first IP will always be localhost; try #1 instead
	// FIXME this sucks
	return strings.Split(addrs[1].String(), "/")[0]
}

func createDNSServer(t *testing.T) *dnsserver.DNSServer {
	server := dnsserver.DNSServer{
		Zones: map[string]*config.Zone{
			Domain + ".": {
				SOA: &dnsconfig.SOA{
					Domain:  Domain + ".",
					Admin:   fmt.Sprintf("administrator.%s.", Domain),
					MinTTL:  60,
					Serial:  1,
					Refresh: 60,
					Retry:   1,
					Expire:  120,
				},
				NS: &dnsconfig.NS{
					Servers: []string{Domain + "."},
					TTL:     60,
				},
				Records: []*config.Record{
					{
						Type: dnsconfig.TypeA,
						Name: Domain + ".",
						Value: &dnsconfig.A{
							Addresses: []net.IP{net.ParseIP(getExternalIP(t))},
							TTL:       60,
						},
					},
				},
			},
		},
	}

	if err := server.Start(getExternalIP(t) + ":" + DNSPort); err != nil {
		t.Fatal(err)
	}

	return &server
}

func createPebble(t *testing.T) {
	dns := createDNSServer(t)

	t.Cleanup(func() {
		dns.Shutdown()
	})

	externalIP := getExternalIP(t)

	d := duct.New(duct.Manifest{
		{
			Name:  "pebble",
			Image: "letsencrypt/pebble:latest",
			PortForwards: map[int]int{
				14000: 14000,
				15000: 15000,
			},
			Command: []string{"/bin/sh", "-c", fmt.Sprintf("pebble -config /test/config/pebble-config.json -strict -dnsserver %s:%s", externalIP, DNSPort)},
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

func createACMEAccount(t *testing.T) *ACMEParams {
	ap := &ACMEParams{
		IgnoreVerify: true,
		Directory:    "https://127.0.0.1:14000/dir",
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

	return ap
}

func serveChallenge(chal acme.Challenge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(chal.KeyAuthorization))
	}
}

func createHTTPServer(t *testing.T, chal acme.Challenge) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc(chal.HTTP01ResourcePath(), serveChallenge(chal))

	srv := &http.Server{Addr: ":5002", Handler: mux}
	go srv.ListenAndServe()

	return srv
}

type httpSolver struct {
	ap  *ACMEParams
	srv *http.Server
	t   *testing.T
}

func (hs *httpSolver) Present(ctx context.Context, chal acme.Challenge) error {
	hs.srv = createHTTPServer(hs.t, chal)
	return nil
}

func (hs *httpSolver) CleanUp(ctx context.Context, chal acme.Challenge) error {
	return hs.srv.Shutdown(ctx)
}

func TestACMEAccount(t *testing.T) {
	createPebble(t)
	createACMEAccount(t)
}

func TestACMECreateCertificate(t *testing.T) {
	createPebble(t)
	ap := createACMEAccount(t)

	if err := ap.GetNewCertificate(context.Background(), Domain, nil); err == nil {
		t.Fatal("Ran anyway without solvers")
	}

	if err := ap.GetNewCertificate(context.Background(), Domain, Solvers{acme.ChallengeTypeHTTP01: &httpSolver{ap: ap, t: t}}); err != nil {
		t.Fatal(err)
	}

	if ap.Account.Certificates == nil || ap.Account.Certificates[Domain] == nil {
		t.Fatal("Certificate was not cached")
	}

	// obtain the certificate with no solvers. should return a valid cert.
	cert, err := ap.GetCertificate(context.Background(), Domain, nil)
	if err != nil {
		t.Fatal(err)
	}

	if cert == nil {
		t.Fatal("cert was nil after cache")
	}
}
