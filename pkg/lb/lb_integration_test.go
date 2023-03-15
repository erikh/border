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

func TestTCPIntegrationNginx(t *testing.T) {
	balancerConfig := BalancerConfig{
		Kind: BalanceTCP,
		Backends: []string{
			// XXX if the nginx's are changed below, these must change too
			"127.0.0.1:8001",
			"127.0.0.1:8002",
			"127.0.0.1:8003",
			"127.0.0.1:8004",
			"127.0.0.1:8005",
		},
		SimultaneousConnections:  65535,
		MaxConnectionsPerAddress: 65535,
	}

	d := duct.New(duct.Manifest{
		{
			// NOTE the other definitions specify "LocalImage" to avoid 5 pulls at
			// once, which bombs out quay.io and fails often. However, if this one is
			// removed, it will not go so well for first-timers.
			Name:         "balancer-1",
			Image:        "quay.io/dockerlibrary/nginx",
			PortForwards: makeNginxPortForward(8001),
			AliveFunc:    makeNginxAliveFunc("localhost:8001"),
		},
		{
			Name:         "balancer-2",
			Image:        "quay.io/dockerlibrary/nginx",
			LocalImage:   true,
			PortForwards: makeNginxPortForward(8002),
			AliveFunc:    makeNginxAliveFunc("localhost:8002"),
		},
		{
			Name:         "balancer-3",
			Image:        "quay.io/dockerlibrary/nginx",
			PortForwards: makeNginxPortForward(8003),
			LocalImage:   true,
			AliveFunc:    makeNginxAliveFunc("localhost:8003"),
		},
		{
			Name:         "balancer-4",
			Image:        "quay.io/dockerlibrary/nginx",
			PortForwards: makeNginxPortForward(8004),
			LocalImage:   true,
			AliveFunc:    makeNginxAliveFunc("localhost:8004"),
		},
		{
			Name:         "balancer-5",
			Image:        "quay.io/dockerlibrary/nginx",
			PortForwards: makeNginxPortForward(8005),
			LocalImage:   true,
			AliveFunc:    makeNginxAliveFunc("localhost:8005"),
		},
	}, duct.WithNewNetwork("load-balancer-test"))

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

	t.Cleanup(func() { balancer.Shutdown() })

	u, err := url.Parse(fmt.Sprintf("http://%s", balancer.listener.Addr()))
	if err != nil {
		t.Fatal(err)
	}

	gen := makeload.LoadGenerator{
		Concurrency:             uint(runtime.NumCPU()),
		SimultaneousConnections: uint(runtime.NumCPU()),
		TotalConnections:        20000,
		URL:                     u,
		Ctx:                     context.Background(),
	}

	before := time.Now()

	if err := gen.Spawn(); err != nil {
		t.Fatal(err)
	}

	t.Logf("RTT in %v", time.Since(before))

	if gen.Stats.Successes != 20000 {
		t.Fatalf("Not all requests were successful: total: %d", gen.Stats.Successes)
	}
}
