package lb

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

type Kind string
type connMap map[string]uint64

const (
	BalanceTCP  = Kind("tcp")
	BalanceHTTP = Kind("http")
)

type Balancer struct {
	listenSpec       string
	kind             Kind
	backendAddresses map[string]struct{}
	backendConns     connMap
	connBuffer       uint
	maxConns         uint64 // per address

	listener   net.Listener
	mutex      sync.RWMutex
	cancelFunc context.CancelFunc
}

func Init(listenSpec string, kind Kind, addresses []string, connBuffer uint, maxConns uint64) *Balancer {
	// FIXME this constructor needs timeout handling

	conns := connMap{}
	addrs := map[string]struct{}{}

	// pregame the map, not strictly necessary but might help keep some bugs at
	// bay.
	for _, addr := range addresses {
		conns[addr] = 0
		addrs[addr] = struct{}{}
	}

	return &Balancer{
		listenSpec:       listenSpec,
		kind:             kind,
		backendAddresses: addrs,
		backendConns:     conns,
		connBuffer:       connBuffer,
		maxConns:         maxConns,
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

func (b *Balancer) BalanceTCP(ctx context.Context, notifyFunc func(error)) {
	go b.dispatchTCP(ctx)
	notifyFunc(nil)
}

func (b *Balancer) dispatchTCP(ctx context.Context) {
	connChan := make(chan net.Conn, b.connBuffer)

	go func() {
		for {
			conn, err := b.listener.Accept()
			if err != nil {
				log.Fatalf("Transient error in Accept, terminating listen. Restart border: %v\n", err)
				return
			}

			connChan <- conn
		}
	}()

	go func() {
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
					if count <= b.maxConns && count < lowestCount {
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

					// FIXME timeouts to prevent slowloris attacks
					// FIXME probably should use CopyN to avoid other styles of slowloris attack (endless data)
					go func() {
						io.Copy(conn, backend)
						b.mutex.Lock()
						if _, ok := b.backendConns[lowestAddr]; ok {
							b.backendConns[lowestAddr]--
						}
						b.mutex.Unlock()
					}()

					go io.Copy(backend, conn)

					b.mutex.Unlock()
				} else {
					goto retry
				}
			}
		}
	}()
}
