package lb

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"runtime"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/erikh/go-makeload"
	"github.com/sirupsen/logrus"
)

type listenFunc func(net.Listener, *atomic.Uint64)

func makeListenFunc(f func(net.Conn), timeTaken *atomic.Int64) listenFunc {
	return func(backend net.Listener, count *atomic.Uint64) {
		for {
			conn, err := backend.Accept()
			if err != nil && !errors.Is(err, net.ErrClosed) {
				logrus.Fatal(err)
			} else if err != nil {
				return
			}

			before := time.Now()
			f(conn)
			conn.Close()
			timeTaken.Add(int64(time.Since(before)))
			count.Add(1)
		}
	}
}

func spawnBackends(t *testing.T, listen listenFunc) ([]string, *atomic.Uint64) {
	count := &atomic.Uint64{}
	backends := []net.Listener{}

	t.Logf("Spawning %d backends", runtime.NumCPU())

	for i := 0; i < runtime.NumCPU(); i++ {
		backend, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err, i)
		}
		backends = append(backends, backend)
	}

	t.Cleanup(func() {
		for _, b := range backends {
			b.Close()
		}
	})

	addresses := []string{}

	for _, b := range backends {
		go listen(b, count)
		addresses = append(addresses, b.Addr().String())
	}

	return addresses, count
}

func makeTCPBalancer(t *testing.T, addresses []string, timeout time.Duration) *Balancer {
	config := BalancerConfig{
		Kind:                     BalanceTCP,
		Backends:                 addresses,
		SimultaneousConnections:  65535,
		MaxConnectionsPerAddress: 65535,
		ConnectionTimeout:        timeout,
	}

	balancer := Init("127.0.0.1:0", config)
	if err := balancer.Start(); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(balancer.Shutdown)

	return balancer
}

func tcpLoadGenerate(t *testing.T, addresses []string, connections uint, timeout time.Duration) makeload.LoadGenerator {
	balancer := makeTCPBalancer(t, addresses, timeout)

	u, err := url.Parse(fmt.Sprintf("http://%s", balancer.listener.Addr()))
	if err != nil {
		t.Fatal(err)
	}

	gen := makeload.LoadGenerator{
		Concurrency:             uint(runtime.NumCPU()),
		SimultaneousConnections: 100,
		TotalConnections:        connections,
		URL:                     u,
		Ctx:                     context.Background(),
	}

	if err := gen.Spawn(); err != nil {
		t.Fatal(err)
	}

	return gen
}

func TestTCPDialErrors(t *testing.T) {
	// hopefully this doesn't fail for too many people
	balancer := makeTCPBalancer(t, []string{"127.0.0.1:1"}, 100*time.Millisecond)

	conn, err := net.Dial("tcp", balancer.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := io.Copy(conn, rand.Reader); !errors.Is(err, syscall.ECONNRESET) {
		t.Fatalf("did not receive 'is closed' error, received %q", err.Error())
	}
}

func TestTCPEndlessData(t *testing.T) {
	timeTaken := &atomic.Int64{}

	addresses, _ := spawnBackends(t, makeListenFunc(func(conn net.Conn) {
		if _, err := io.Copy(io.Discard, conn); err != nil {
			logrus.Fatal(err)
			return
		}
	}, timeTaken))

	balancer := makeTCPBalancer(t, addresses, 100*time.Millisecond)

	conn, err := net.Dial("tcp", balancer.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := io.Copy(conn, rand.Reader); !errors.Is(err, syscall.EPIPE) && !errors.Is(err, io.ErrClosedPipe) && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("did not receive 'is closed' error, received %q", err.Error())
	}
}

func TestTCPDialError(t *testing.T) {
	const connections = 1000
	timeTaken := &atomic.Int64{}

	addresses, count := spawnBackends(t, makeListenFunc(func(conn net.Conn) {}, timeTaken))
	gen := tcpLoadGenerate(t, addresses, connections, 0)

	if gen.Stats.Failures != connections {
		t.Fatalf("Requests succeeded when they shouldn't have total failures: %d, successes: %d", gen.Stats.Failures, gen.Stats.Successes)
	}

	t.Logf("Average failure time: %v", time.Duration(uint64(timeTaken.Load())/count.Load()))
}

func TestTCPTimeout(t *testing.T) {
	const connections = 1000
	timeTaken := &atomic.Int64{}

	listen := makeListenFunc(func(conn net.Conn) {
		if _, err := io.Copy(io.Discard, conn); err != nil {
			logrus.Fatal(err)
			return
		}
	}, timeTaken)

	addresses, count := spawnBackends(t, listen)
	gen := tcpLoadGenerate(t, addresses, connections, 10*time.Millisecond)

	if gen.Stats.Failures != connections {
		t.Fatalf("Requests succeeded when they shouldn't have total failures: %d, successes: %d", gen.Stats.Failures, gen.Stats.Successes)
	}

	t.Logf("Average failure time: %v", time.Duration(uint64(timeTaken.Load())/count.Load()))
}

func TestTCP(t *testing.T) {
	const connections = 100000
	timeTaken := &atomic.Int64{}

	addresses, count := spawnBackends(t, makeListenFunc(func(conn net.Conn) {}, timeTaken))
	gen := tcpLoadGenerate(t, addresses, connections, 0)

	if gen.Stats.Failures != connections {
		t.Fatalf("Requests succeeded when they shouldn't have total failures: %d, successes: %d", gen.Stats.Failures, gen.Stats.Successes)
	}

	if count.Load() != connections {
		t.Fatalf("Not all requests were delivered: total: %d", count)
	}

	t.Logf("Average RTT for %d connections: %v", connections, time.Duration(uint64(timeTaken.Load())/count.Load()))
}
