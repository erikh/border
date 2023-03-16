package lb

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/erikh/go-makeload"
)

type httpCounter struct {
	count atomic.Uint64
}

func (h *httpCounter) serveHTTP(w http.ResponseWriter, r *http.Request) {
	h.count.Add(1)
}

func (h *httpCounter) makeHTTPBackend() (*http.Server, net.Listener, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.serveHTTP)

	server := &http.Server{
		Handler: mux,
	}

	errChan := make(chan error, 1)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("while booting listener: %w", err)
	}

	go func() {
		if err := server.Serve(l); err != nil {
			errChan <- fmt.Errorf("while booting backend server: %w", err)
		}
	}()

	select {
	case <-time.After(100 * time.Millisecond): // XXX make this adjustable?
		return server, l, nil
	case err := <-errChan:
		return nil, nil, err
	}
}

func TestHTTP(t *testing.T) {
	counter := &httpCounter{}
	backends := []*http.Server{}
	listeners := []net.Listener{}

	t.Logf("Spawning %d backends", runtime.NumCPU())

	for i := 0; i < runtime.NumCPU(); i++ {
		backend, listener, err := counter.makeHTTPBackend()
		if err != nil {
			t.Fatal(err)
		}

		backends = append(backends, backend)
		listeners = append(listeners, listener)
	}

	t.Cleanup(func() {
		for _, b := range backends {
			if err := b.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
		}

		for _, l := range listeners {
			l.Close()
		}
	})

	addresses := []string{}

	for _, l := range listeners {
		addresses = append(addresses, l.Addr().String())
	}

	config := BalancerConfig{
		Kind:                     BalanceHTTP,
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

	if counter.count.Load() != 100000 {
		t.Fatalf("Not all requests were delivered: total: %d", counter.count.Load())
	}
}
