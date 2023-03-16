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
		makeNginx("balancer-1", false, 8001),
		makeNginx("balancer-2", true, 8002),
		makeNginx("balancer-3", true, 8003),
		makeNginx("balancer-4", true, 8004),
		makeNginx("balancer-5", true, 8005),
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
