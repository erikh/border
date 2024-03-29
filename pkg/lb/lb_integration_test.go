package lb

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"runtime"
	"testing"
	"time"

	"github.com/erikh/duct"
	"github.com/erikh/go-makeload"
	dc "github.com/fsouza/go-dockerclient"
)

func makeNginxPortForward(port int) map[int]int {
	return map[int]int{
		port: 80,
	}
}

func makeNginxAliveFunc(listenSpec string) func(context.Context, *dc.Client, string) error {
	return func(context.Context, *dc.Client, string) error {
		for {
			if _, err := net.Dial("tcp", listenSpec); err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			return nil
		}
	}
}

func makeNginx(name string, localImage bool, port int) *duct.Container {
	return &duct.Container{
		Name:         name,
		Image:        "quay.io/dockerlibrary/nginx",
		LocalImage:   localImage,
		PortForwards: makeNginxPortForward(port),
		AliveFunc:    makeNginxAliveFunc(localhost(port)),
	}
}

func localhost(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}

func TestTCPIntegrationNginx(t *testing.T) {
	balancerConfig := BalancerConfig{
		Kind: BalanceTCP,
		Backends: []string{
			// XXX if the nginx's are changed below, these must change too
			localhost(8001),
			localhost(8002),
			localhost(8003),
			localhost(8004),
			localhost(8005),
		},
		SimultaneousConnections:  65535,
		MaxConnectionsPerAddress: 65535,
		ConnectionTimeout:        30 * time.Second,
	}

	d := duct.New(duct.Manifest{
		// NOTE the other definitions specify "LocalImage" to avoid 5 pulls at
		// once, which bombs out quay.io and fails often. However, if this one is
		// added as a LocalImage, it will not go so well for first-timers.
		makeNginx("tcp-balancer-1", false, 8001),
		makeNginx("tcp-balancer-2", true, 8002),
		makeNginx("tcp-balancer-3", true, 8003),
		makeNginx("tcp-balancer-4", true, 8004),
		makeNginx("tcp-balancer-5", true, 8005),
	}, duct.WithNewNetwork("tcp-load-balancer-test"))

	d.HandleSignals(true)

	t.Cleanup(func() {
		if err := d.Teardown(context.Background()); err != nil {
			t.Fatal(err)
		}
	})

	if err := d.Launch(context.Background()); err != nil {
		t.Fatal(err)
	}

	balancer := Init("127.0.0.1:0", balancerConfig)
	if err := balancer.Start(); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(balancer.Shutdown)

	u, err := url.Parse(fmt.Sprintf("http://%s", balancer.listener.Addr()))
	if err != nil {
		t.Fatal(err)
	}

	var (
		totalCount  = uint(20000)
		concurrency = uint(runtime.NumCPU())
	)

	gen := makeload.LoadGenerator{
		Concurrency:             concurrency,
		SimultaneousConnections: concurrency,
		TotalConnections:        totalCount,
		URL:                     u,
		Ctx:                     context.Background(),
	}

	before := time.Now()

	if err := gen.Spawn(); err != nil {
		t.Fatal(err)
	}

	t.Logf("RTT in %v: %v connections, %v concurrency", time.Since(before), totalCount, concurrency)

	if gen.Stats.Successes != totalCount {
		t.Fatalf("Not all requests were successful: total: %d", gen.Stats.Successes)
	}
}

func TestHTTPIntegrationNginx(t *testing.T) {
	balancerConfig := BalancerConfig{
		Kind: BalanceHTTP,
		Backends: []string{
			// XXX if the nginx's are changed below, these must change too
			localhost(8011),
			localhost(8012),
			localhost(8013),
			localhost(8014),
			localhost(8015),
		},
		SimultaneousConnections:  65535,
		MaxConnectionsPerAddress: 65535,
		ConnectionTimeout:        30 * time.Second,
	}

	d := duct.New(duct.Manifest{
		// NOTE the other definitions specify "LocalImage" to avoid 5 pulls at
		// once, which bombs out quay.io and fails often. However, if this one is
		// added as a LocalImage, it will not go so well for first-timers.
		makeNginx("http-balancer-1", false, 8011),
		makeNginx("http-balancer-2", true, 8012),
		makeNginx("http-balancer-3", true, 8013),
		makeNginx("http-balancer-4", true, 8014),
		makeNginx("http-balancer-5", true, 8015),
	}, duct.WithNewNetwork("http-load-balancer-test"))

	d.HandleSignals(true)

	t.Cleanup(func() {
		if err := d.Teardown(context.Background()); err != nil {
			t.Fatal(err)
		}
	})

	if err := d.Launch(context.Background()); err != nil {
		t.Fatal(err)
	}

	balancer := Init("127.0.0.1:0", balancerConfig)
	if err := balancer.Start(); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(balancer.Shutdown)

	u, err := url.Parse(fmt.Sprintf("http://%s", balancer.listener.Addr()))
	if err != nil {
		t.Fatal(err)
	}

	var (
		totalCount  = uint(20000)
		concurrency = uint(runtime.NumCPU())
	)

	gen := makeload.LoadGenerator{
		Concurrency:             concurrency,
		SimultaneousConnections: concurrency,
		TotalConnections:        totalCount,
		URL:                     u,
		Ctx:                     context.Background(),
	}

	before := time.Now()

	if err := gen.Spawn(); err != nil {
		t.Fatal(err)
	}

	t.Logf("RTT in %v: %v connections, %v concurrency", time.Since(before), totalCount, concurrency)

	if gen.Stats.Successes != totalCount {
		t.Fatalf("Not all requests were successful: total: %d", gen.Stats.Successes)
	}
}
