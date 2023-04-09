package acmekit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/erikh/border/integration-tests"
	"github.com/erikh/border/pkg/acmekit"
	"github.com/erikh/duct"
	"github.com/mholt/acmez/acme"
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

func createPebble(t *testing.T) {
	externalIP := getExternalIP(t)

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
			ExtraHosts: map[string][]string{
				externalIP: {"example.org"},
			},
		},
		{
			Name:  "pebble-challtestsrv",
			Image: "letsencrypt/pebble-challtestsrv:latest",
			PortForwards: map[int]int{
				8055: 8055,
			},
			Command: []string{"/bin/sh", "-c", `pebble-challtestsrv -defaultIPv6 "" -defaultIPv4 10.30.50.3`},
			IPv4:    "10.30.50.3",
			ExtraHosts: map[string][]string{
				externalIP: {"example.org"},
			},
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

func createACMEAccount(t *testing.T) *acmekit.ACMEParams {
	directory, err := url.Parse("https://127.0.0.1:14000/dir")
	if err != nil {
		t.Fatal(err)
	}

	ap := &acmekit.ACMEParams{
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

	return ap
}

func serveChallenge(chal acme.Challenge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("got here")
		w.Write([]byte(chal.KeyAuthorization))
	}
}

func createHTTPServer(t *testing.T, chal acme.Challenge) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc(chal.HTTP01ResourcePath(), serveChallenge(chal))

	srv := &http.Server{Handler: mux}
	go srv.ListenAndServe()

	return srv
}

type httpSolver struct {
	ap  *acmekit.ACMEParams
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
	integration.RequireDocker(t)

	createPebble(t)
	createACMEAccount(t)
}

func TestACMECreateCertificate(t *testing.T) {
	integration.RequireDocker(t)

	const domain = "example.org"

	createPebble(t)
	ap := createACMEAccount(t)

	if err := ap.GetNewCertificate(context.Background(), domain, nil); err == nil {
		t.Fatal("Ran anyway without solvers")
	}

	if err := ap.GetNewCertificate(context.Background(), domain, acmekit.Solvers{acme.ChallengeTypeHTTP01: &httpSolver{ap: ap, t: t}}); err != nil {
		t.Fatal(err)
	}

	if ap.Account.Certificates == nil || ap.Account.Certificates[domain] == nil {
		t.Fatal("Certificate was not cached")
	}
}
