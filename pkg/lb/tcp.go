package lb

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
)

func (b *Balancer) BalanceTCP(ctx context.Context, notifyFunc func(error)) {
	go b.monitorListen(ctx)
	go b.dispatchTCP(ctx)
	notifyFunc(nil)
}

func (b *Balancer) dispatchTCP(ctx context.Context) {
	connChan := make(chan net.Conn, b.connBuffer)

	go b.acceptTCPConns(connChan)
	go b.forwardTCPConn(ctx, connChan)
}

func (b *Balancer) acceptTCPConns(connChan chan net.Conn) {
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

func (b *Balancer) forwardTCPConn(ctx context.Context, connChan chan net.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		case conn := <-connChan:
			var (
				connCtx context.Context
				cancel  context.CancelFunc
			)

			if b.timeout != 0 {
				connCtx, cancel = context.WithTimeout(ctx, b.timeout)
			} else {
				connCtx, cancel = context.WithCancel(ctx)
			}

			go b.closeConn(connCtx, conn)

			// find the lowest count of conns in the group. If all hosts are
			// saturated, loop until that changes.
		retry:
			lowestAddr := b.getLowestBalancer()

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

				go b.closeConn(connCtx, backend)
				go b.decrementCount(connCtx, lowestAddr)

				// FIXME timeouts to prevent slowloris attacks. Also shutdown socket on context finish.
				// FIXME probably should use CopyN to avoid other styles of slowloris attack (endless data)
				go func() {
					io.Copy(backend, conn)
					cancel()
				}()

				go func() {
					io.Copy(conn, backend)
					cancel()
				}()

				b.mutex.Unlock()
			} else {
				goto retry
			}
		}
	}
}
