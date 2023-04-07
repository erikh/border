package lb

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type connMap map[string]int

const (
	BalanceTCP  = "tcp"
	BalanceHTTP = "http"
)

type TLSBalancerConfig struct {
	CACertificate []byte
	Certificate   []byte
	Key           []byte
}

type BalancerConfig struct {
	Kind                     string
	Backends                 []string
	SimultaneousConnections  int
	MaxConnectionsPerAddress int
	ConnectionTimeout        time.Duration
	TLS                      *TLSBalancerConfig
}

type Balancer struct {
	listenSpec       string
	kind             string
	backendAddresses map[string]struct{}
	backendConns     connMap
	connBuffer       int
	maxConns         int           // per address
	timeout          time.Duration // per connection

	tlsConfig  *tls.Config
	listener   net.Listener
	listenerIP string
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

	host, _, err := net.SplitHostPort(listenSpec)
	if err != nil {
		logrus.Fatalf("Invalid address in listener %q: %v", listenSpec, err)
	}

	return &Balancer{
		listenerIP:       host,
		listenSpec:       listenSpec,
		kind:             config.Kind,
		backendAddresses: addrs,
		backendConns:     conns,
		connBuffer:       config.SimultaneousConnections,
		maxConns:         config.MaxConnectionsPerAddress,
		timeout:          config.ConnectionTimeout,
		tlsConfig:        makeTLSConfig(config),
	}
}

func (b *Balancer) Start() error {
	var (
		listener net.Listener
		err      error
	)

	if b.tlsConfig != nil {
		listener, err = tls.Listen("tcp", b.listenSpec, b.tlsConfig)
	} else {
		listener, err = net.Listen("tcp", b.listenSpec)
	}

	if err != nil {
		return fmt.Errorf("Error enabling load balancer: %w", err)
	}

	b.listener = listener
	ctx, cancel := context.WithCancel(context.Background())
	b.cancelFunc = cancel

	errChan := make(chan error, 1)

	switch b.kind {
	case BalanceTCP:
		go b.BalanceTCP(ctx, func(err error) {
			errChan <- err
		})

		if err := <-errChan; err != nil {
			b.listener.Close()
			cancel()
			return err
		}

		return nil
	case BalanceHTTP:
		go b.BalanceHTTP(ctx, func(err error) {
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

func makeTLSConfig(config BalancerConfig) *tls.Config {
	var tlsConfig *tls.Config

	if config.TLS != nil {
		var cacert []byte

		if config.TLS.CACertificate != nil {
			block, _ := pem.Decode(config.TLS.CACertificate)
			if block != nil {
				cacert = block.Bytes
			} else {
				logrus.Fatalf("Could not parse CA certificate as provided, is it formatted correctly?")
			}
		}

		cert, err := tls.X509KeyPair(config.TLS.Certificate, config.TLS.Key)
		if err != nil {
			logrus.Fatalf("Could not load TLS certificate pair: %v", err)
		}

		if cacert != nil {
			cert.Certificate = append(cert.Certificate, cacert)
		}

		tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	}

	return tlsConfig
}

func (b *Balancer) closeConn(ctx context.Context, conn net.Conn) {
	<-ctx.Done()
	conn.Close()
}

func (b *Balancer) getLowestBalancer() string {
	var lowestAddr string
	var lowestCount int

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

	return lowestAddr
}

func (b *Balancer) decrementCount(ctx context.Context, lowestAddr string) {
	<-ctx.Done()

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
