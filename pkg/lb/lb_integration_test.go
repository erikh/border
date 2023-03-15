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
			Name:  "balancer-1",
			Image: "quay.io/dockerlibrary/nginx",
			PortForwards: map[int]int{
				8001: 80,
			},
			AliveFunc: func(ctx context.Context, client *dc.Client, id string) error {
				for {
					if _, err := net.Dial("tcp", "localhost:8001"); err != nil {
						time.Sleep(100 * time.Millisecond)
						continue
					}

					break
				}
				return nil
			},
		},
		{
			Name:  "balancer-2",
			Image: "quay.io/dockerlibrary/nginx",
			PortForwards: map[int]int{
				8002: 80,
			},
			LocalImage: true,
			AliveFunc: func(ctx context.Context, client *dc.Client, id string) error {
				for {
					if _, err := net.Dial("tcp", "localhost:8002"); err != nil {
						time.Sleep(100 * time.Millisecond)
						continue
					}

					break
				}
				return nil
			},
		},
		{
			Name:  "balancer-3",
			Image: "quay.io/dockerlibrary/nginx",
			PortForwards: map[int]int{
				8003: 80,
			},
			LocalImage: true,
			AliveFunc: func(ctx context.Context, client *dc.Client, id string) error {
				for {
					if _, err := net.Dial("tcp", "localhost:8003"); err != nil {
						time.Sleep(100 * time.Millisecond)
						continue
					}

					break
				}
				return nil
			},
		},
		{
			Name:  "balancer-4",
			Image: "quay.io/dockerlibrary/nginx",
			PortForwards: map[int]int{
				8004: 80,
			},
			LocalImage: true,
			AliveFunc: func(ctx context.Context, client *dc.Client, id string) error {
				for {
					if _, err := net.Dial("tcp", "localhost:8004"); err != nil {
						time.Sleep(100 * time.Millisecond)
						continue
					}

					break
				}
				return nil
			},
		},
		{
			Name:  "balancer-5",
			Image: "quay.io/dockerlibrary/nginx",
			PortForwards: map[int]int{
				8005: 80,
			},
			LocalImage: true,
			AliveFunc: func(ctx context.Context, client *dc.Client, id string) error {
				for {
					if _, err := net.Dial("tcp", "localhost:8005"); err != nil {
						time.Sleep(100 * time.Millisecond)
						continue
					}

					break
				}
				return nil
			},
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
