package lb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
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
}

type Balancer struct {
	listenSpec       string
	kind             string
	backendAddresses map[string]struct{}
	backendConns     connMap
	connBuffer       uint
	maxConns         uint64 // per address

	listener   net.Listener
	mutex      sync.RWMutex
	cancelFunc context.CancelFunc
}

func Init(listenSpec string, config BalancerConfig) *Balancer {
	// FIXME this constructor needs timeout handling

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

func (b *Balancer) BalanceTCP(ctx context.Context, notifyFunc func(error)) {
	go b.monitorListen(ctx)
	go b.dispatchTCP(ctx)
	notifyFunc(nil)
}

func (b *Balancer) monitorListen(ctx context.Context) {
	<-ctx.Done()
	b.listener.Close()
}

func (b *Balancer) dispatchTCP(ctx context.Context) {
	connChan := make(chan net.Conn, b.connBuffer)

	go b.acceptConns(connChan)
	go b.forwardConn(ctx, connChan)
}

func (b *Balancer) acceptConns(connChan chan net.Conn) {
	for {
		conn, err := b.listener.Accept()
		if err != nil && !errors.Is(err, net.ErrClosed) {
			log.Fatalf("Transient error in Accept, terminating listen. Restart border: %v", err)
			return
		} else if err != nil {
			return
		}

		connChan <- conn
	}
}

func (b *Balancer) forwardConn(ctx context.Context, connChan chan net.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		case conn := <-connChan:
			// find the lowest count of conns in the group. If all hosts are
			// saturated, loop until that changes.
		retry:
			var lowestAddr string
			var lowestCount uint64

			b.mutex.RLock()
			for addr := range b.backendAddresses {
				count := b.backendConns[addr]
				if lowestAddr == "" && count < b.maxConns {
					lowestAddr = addr
					lowestCount = count
				} else if count < b.maxConns && count <= lowestCount {
					lowestAddr = addr
					lowestCount = count
				}
			}
			b.mutex.RUnlock()

			// if we have a designated lowest count address, dial the backend and
			// schedule the copy. Remove the backend from the pool on any error.
			if lowestAddr != "" {
				b.mutex.Lock()
				b.backendConns[lowestAddr]++
				backend, err := net.Dial("tcp", lowestAddr)

				// FIXME pool removal should happen as a result of health checks,
				// not dial errors. We should try with a different address on dial
				// errors, which we do by pruning the pool right now. Something
				// more elegant should be employed in the face of transient errors.
				if err != nil {
					log.Printf("Backend %q failed: removing from pool: %v", lowestAddr, err)
					delete(b.backendAddresses, lowestAddr)
					delete(b.backendConns, lowestAddr)
					b.mutex.Unlock()
					goto retry
				}

				// FIXME timeouts to prevent slowloris attacks. Also shutdown socket on context finish.
				// FIXME probably should use CopyN to avoid other styles of slowloris attack (endless data)
				go func() {
					io.Copy(backend, conn)
					conn.Close()
					backend.Close()

					b.mutex.Lock()
					if _, ok := b.backendConns[lowestAddr]; ok {
						b.backendConns[lowestAddr]--
					}
					b.mutex.Unlock()
				}()

				go func() {
					io.Copy(conn, backend)
					conn.Close()
					backend.Close()
				}()

				b.mutex.Unlock()
			} else {
				goto retry
			}
		}
	}
}
