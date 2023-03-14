package lb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"runtime"
	"sync"
	"testing"

	"github.com/erikh/go-makeload"
)

func TestTCP(t *testing.T) {
	var count uint64
	var mutex sync.Mutex
	backends := []net.Listener{}

	t.Logf("Spawning %d backends", runtime.NumCPU())

	for i := 0; i < runtime.NumCPU(); i++ {
		backend, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		backends = append(backends, backend)
	}

	t.Cleanup(func() {
		for _, b := range backends {
			b.Close()
		}
	})

	listen := func(backend net.Listener, count *uint64, mutex *sync.Mutex) {
		for {
			conn, err := backend.Accept()
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Fatal(err)
			} else if err != nil {
				return
			}

			conn.Close()
			mutex.Lock()
			(*count)++
			mutex.Unlock()
		}
	}

	addresses := []string{}

	for _, b := range backends {
		go listen(b, &count, &mutex)
		addresses = append(addresses, b.Addr().String())
	}

	config := BalancerConfig{
		Kind:                     BalanceTCP,
		Backends:                 addresses,
		SimultaneousConnections:  65535,
		MaxConnectionsPerAddress: 65535,
	}

	balancer := Init("127.0.0.1:0", config)
	if err := balancer.Start(); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { balancer.Shutdown() })

	u, err := url.Parse(fmt.Sprintf("http://%s", balancer.listener.Addr()))
	if err != nil {
		t.Fatal(err)
	}

	gen := makeload.LoadGenerator{
		Concurrency:             uint(len(backends)),
		SimultaneousConnections: 1000,
		TotalConnections:        100000,
		URL:                     u,
		Ctx:                     context.Background(),
	}

	if err := gen.Spawn(); err != nil {
		t.Fatal(err)
	}

	if count != 100000 {
		t.Fatalf("Not all requests were delivered: total: %d", count)
	}
}
