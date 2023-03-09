package controlserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/erikh/border/pkg/config"
	"github.com/go-jose/go-jose/v3"
)

const (
	nonceSize       = 128
	NonceExpiration = 30 * time.Second // passed in to the Start() function, to make tests easier
)

type Server struct {
	server   *http.Server
	listener net.Listener
	config   config.Config

	nonces map[string]time.Time
	// for testing performance; this just comes from a constant normally, but to
	// wait in tests for a long time seems like a waste of everyone's time
	expireTime  time.Duration
	nonceMutex  sync.RWMutex
	configMutex sync.RWMutex

	cancelSupervision context.CancelFunc
}

// Start the control server in the background.
//
// We assume after bootWait, with no errors, the server has started. I can't
// see a better way to do this with net/http since there is not a notifier
// callback. Would prefer a better way to launch the server without blocking.
func Start(config config.Config, listenSpec string, expireTime, bootWait time.Duration) (*Server, error) {
	errChan := make(chan error, 1)

	ctx, cancel := context.WithCancel(context.Background())

	// we use a listener directly so we can use ":0" in tests with ease.
	l, err := net.Listen("tcp", listenSpec)
	if err != nil {
		return nil, err
	}

	server := &Server{
		listener:          l,
		nonces:            map[string]time.Time{},
		config:            config,
		cancelSupervision: cancel,
		expireTime:        expireTime,
	}

	s := &http.Server{Handler: server.configureMux()}

	go server.expireNonces(ctx)
	go func() {
		errChan <- s.Serve(l)
	}()

	server.server = s

	select {
	case err := <-errChan:
		return nil, err
	case <-time.After(bootWait):
		return server, nil
	}
}

// Shutdown the server. Accept a context for timing out the shutdown process.
func (s *Server) Shutdown(ctx context.Context) error {
	// idea of the defer is to cancel supervision after shutdown, to avoid a network race
	defer s.cancelSupervision()
	defer s.listener.Close()
	return s.server.Shutdown(ctx)
}

func (s *Server) expireNonces(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(s.expireTime)
		}

		for n, t := range s.nonces {
			s.nonceMutex.RLock()
			if t.Before(time.Now().Add(-s.expireTime)) {
				s.nonceMutex.RUnlock()
				s.nonceMutex.Lock()
				delete(s.nonces, n)
				s.nonceMutex.Unlock()
				continue // XXX avoid the double unlock below
			}
			s.nonceMutex.RUnlock()
		}
	}
}

func (s *Server) getEncrypter() (jose.Encrypter, error) {
	return jose.NewEncrypter(jose.A256GCM, jose.Recipient{Algorithm: jose.A256KW, Key: s.config.AuthKey}, nil)
}

func (s *Server) configureMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/nonce", s.handleNonce)
	mux.HandleFunc("/authCheck", s.handleAuthCheck)
	mux.HandleFunc("/configUpdate", s.handleConfigUpdate)
	mux.HandleFunc("/peerRegister", s.handlePeerRegister)
	return mux
}

func (s *Server) saveConfig(w http.ResponseWriter) {
	if err := s.config.Save(); err != nil {
		http.Error(w, fmt.Sprintf("Could not save configuration: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) validateNonce(nonce string) error {
	s.nonceMutex.RLock()
	t, ok := s.nonces[nonce]
	s.nonceMutex.RUnlock()
	if !ok {
		return errors.New("Nonce provided does not exist")
	}

	if t.Before(time.Now().Add(-s.expireTime)) {
		return errors.New("Nonce has expired")
	}

	s.nonceMutex.Lock()
	delete(s.nonces, string(nonce))
	s.nonceMutex.Unlock()

	return nil
}
