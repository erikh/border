package lb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/erikh/go-makeload"
)

func TestTCPTimeout(t *testing.T) {
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

	var failureTime time.Duration

	listen := func(backend net.Listener, count *uint64, mutex *sync.Mutex) {
		for {
			conn, err := backend.Accept()
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Fatal(err)
			} else if err != nil {
				return
			}

			before := time.Now()
			io.Copy(io.Discard, conn)

			conn.Close()

			mutex.Lock()
			(*count)++
			mutex.Unlock()

			failureTime += time.Since(before)
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
		ConnectionTimeout:        10 * time.Millisecond,
	}

	balancer := Init("127.0.0.1:0", config)
	if err := balancer.Start(); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(balancer.Shutdown)

	u, err := url.Parse(fmt.Sprintf("http://%s", balancer.listener.Addr()))
	if err != nil {
		t.Fatal(err)
	}

	gen := makeload.LoadGenerator{
		Concurrency:             uint(len(backends)),
		SimultaneousConnections: 100,
		TotalConnections:        1000,
		URL:                     u,
		Ctx:                     context.Background(),
	}

	if err := gen.Spawn(); err != nil {
		t.Fatal(err)
	}

	if gen.Stats.Failures != 1000 {
		t.Fatalf("Requests succeeded when they shouldn't have total failures: %d, successes: %d", gen.Stats.Failures, gen.Stats.Successes)
	}

	t.Logf("Average failure time: %v", time.Duration(float64(failureTime)/float64(count)))
}

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

	t.Cleanup(balancer.Shutdown)

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
