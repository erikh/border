package lb

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

type connMap map[string]uint64

const (
	BalanceTCP  = "tcp"
	BalanceHTTP = "http"
)

type BalancerConfig struct {
	Kind                     string
	Backends                 []string
	SimultaneousConnections  uint
	MaxConnectionsPerAddress uint64
	ConnectionTimeout        time.Duration
}

type Balancer struct {
	listenSpec       string
	kind             string
	backendAddresses map[string]struct{}
	backendConns     connMap
	connBuffer       uint
	maxConns         uint64        // per address
	timeout          time.Duration // per connection

	listener   net.Listener
	mutex      sync.RWMutex
	cancelFunc context.CancelFunc
}

func Init(listenSpec string, config BalancerConfig) *Balancer {
	conns := connMap{}
	addrs := map[string]struct{}{}

	// pre-game the map, not strictly necessary but might help keep some bugs at
	// bay.
	for _, addr := range config.Backends {
		conns[addr] = 0
		addrs[addr] = struct{}{}
	}

	return &Balancer{
		listenSpec:       listenSpec,
		kind:             config.Kind,
		backendAddresses: addrs,
		backendConns:     conns,
		connBuffer:       config.SimultaneousConnections,
		maxConns:         config.MaxConnectionsPerAddress,
		timeout:          config.ConnectionTimeout,
	}
}

func (b *Balancer) Start() error {
	switch b.kind {
	case BalanceTCP:
		listener, err := net.Listen("tcp", b.listenSpec)
		if err != nil {
			return fmt.Errorf("Error enabling load balancer: %w", err)
		}

		b.listener = listener
		ctx, cancel := context.WithCancel(context.Background())
		b.cancelFunc = cancel

		errChan := make(chan error, 1)

		go b.BalanceTCP(ctx, func(err error) {
			errChan <- err
		})

		if err := <-errChan; err != nil {
			b.listener.Close()
			cancel()
			return err
		}

		return nil
	default:
		return fmt.Errorf("Balancer type %q is unsupported", b.kind)
	}
}

func (b *Balancer) Shutdown() {
	b.cancelFunc()
}

func (b *Balancer) closeConn(ctx context.Context, conn net.Conn) {
	select {
	case <-ctx.Done():
	}

	conn.Close()
}

func (b *Balancer) decrementCount(ctx context.Context, lowestAddr string) {
	select {
	case <-ctx.Done():
	}

	b.mutex.Lock()
	if _, ok := b.backendConns[lowestAddr]; ok {
		b.backendConns[lowestAddr]--
	}
	b.mutex.Unlock()
}

func (b *Balancer) monitorListen(ctx context.Context) {
	<-ctx.Done()
	b.listener.Close()
}
