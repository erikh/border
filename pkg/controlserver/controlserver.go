package controlserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/config"
	"github.com/erikh/go-hashchain"
	"github.com/go-jose/go-jose/v3"
	"github.com/sirupsen/logrus"
)

const (
	NonceExpiration = 30 * time.Second // passed in to the Start() function, to make tests easier
)

type Handler func(req api.Request) (api.Message, error)

type Server struct {
	server   *http.Server
	listener net.Listener
	config   *config.Config

	me       *config.Peer
	bootTime time.Time

	debugPayload bool

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
func Start(config *config.Config, me *config.Peer, listenSpec string, expireTime, bootWait time.Duration) (*Server, error) {
	if config.AuthKey == nil {
		return nil, errors.New("You must provide an auth_key to start the server")
	}

	errChan := make(chan error, 1)

	// we use a listener directly so we can use ":0" in tests with ease.
	l, err := net.Listen("tcp", listenSpec)
	if err != nil {
		return nil, err
	}

	debug := os.Getenv("DEBUG_LOG") != ""
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{
		listener:          l,
		nonces:            map[string]time.Time{},
		config:            config,
		cancelSupervision: cancel,
		expireTime:        expireTime,
		bootTime:          time.Now(),
		me:                me,
		debugPayload:      debug && os.Getenv("DEBUG_LOG_PAYLOAD") != "",
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
	s.listener.Close()
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

func (s *Server) makeHandlerFunc(mux *http.ServeMux, method string, req api.Request, key *jose.JSONWebKey, f Handler) {
	mux.HandleFunc("/"+req.Endpoint(), s.handle(method, req.New(), key, f))
}

func (s *Server) handle(method string, req api.Request, key *jose.JSONWebKey, f Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logrus.Debug(req.Endpoint())

		switch method {
		case http.MethodPut:
			if s.debugPayload {
				m, err := req.Marshal()
				if err != nil {
					http.Error(w, fmt.Sprintf("Error marshaling request: %v", err), http.StatusInternalServerError)
					return
				}
				logrus.Debugf("Payload %q", string(m))
			}

			if code, err := s.handleValidateNonce(r, req, key); err != nil {
				http.Error(w, fmt.Sprintf("Nonce validation failed: %v", err), code)
				return
			}
		}

		resp, err := f(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error in request: %v", err), http.StatusInternalServerError)
			return
		}

		serialized, err := api.EncryptResponse(key, resp)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error encrypting response: %v", err), http.StatusInternalServerError)
			return
		}

		w.Write([]byte(serialized)) // nolint:errcheck
	}
}

func (s *Server) configureMux() *http.ServeMux {
	mux := http.NewServeMux()

	// CLI client methods
	s.makeHandlerFunc(mux, http.MethodGet, &api.NonceRequest{}, s.config.AuthKey, s.handleNonce)
	s.makeHandlerFunc(mux, http.MethodPut, api.AuthCheck{}, s.config.AuthKey, s.handleAuthCheck)
	s.makeHandlerFunc(mux, http.MethodPut, &api.ConfigUpdateRequest{}, s.config.AuthKey, s.handleConfigUpdate)
	s.makeHandlerFunc(mux, http.MethodPut, &api.ConfigReloadRequest{}, s.config.AuthKey, s.handleConfigReload)
	s.makeHandlerFunc(mux, http.MethodPut, &api.PeerRegistrationRequest{}, s.config.AuthKey, s.handlePeerRegister)
	s.makeHandlerFunc(mux, http.MethodPut, &api.IdentifyPublisherRequest{}, s.config.AuthKey, s.handleIdentifyPublisher)

	// peer to peer client methods
	s.makeHandlerFunc(mux, http.MethodGet, &api.PeerNonceRequest{}, s.me.Key, s.handleNonce)
	s.makeHandlerFunc(mux, http.MethodPut, &api.UptimeRequest{}, s.me.Key, s.handleUptime)
	s.makeHandlerFunc(mux, http.MethodPut, &api.PingRequest{}, s.me.Key, s.handlePing)
	s.makeHandlerFunc(mux, http.MethodPut, &api.ConfigChainRequest{}, s.me.Key, s.handleConfigChain)
	s.makeHandlerFunc(mux, http.MethodPut, &api.ConfigFetchRequest{}, s.me.Key, s.handleConfigFetch)

	// peer to peer ACME methods
	s.makeHandlerFunc(mux, http.MethodPut, &api.ACMEChallengeRequest{}, s.me.Key, s.handleACMEChallenge)
	s.makeHandlerFunc(mux, http.MethodPut, &api.ACMEReadyRequest{}, s.me.Key, s.handleACMEReady)

	return mux
}

func (s *Server) ReplaceConfig(newConfig *config.Config, newChain *hashchain.Chain) error {
	s.configMutex.Lock()
	newConfig.SetChain(newChain)
	s.config.CopyFrom(newConfig)
	s.configMutex.Unlock()

	return s.saveConfig()
}

func (s *Server) saveConfig() error {
	if err := s.config.Save(); err != nil {
		return fmt.Errorf("Could not save configuration: %v", err)
	}

	if err := s.config.Reload(); err != nil {
		return fmt.Errorf("While reloading configuration: %v", err)
	}

	return nil
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

// handlePut deserializes a JWE request, which should be serviced via the PUT HTTP verb.
func (s *Server) handlePut(r *http.Request, key *jose.JSONWebKey) ([]byte, error) {
	if r.Method != http.MethodPut {
		return nil, errors.New("Invalid HTTP Method for Request")
	}

	byt, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("Could not read body: %w", err)
	}

	o, err := jose.ParseEncrypted(string(byt))
	if err != nil {
		return nil, fmt.Errorf("Could not parse JWE request: %w", err)
	}

	s.configMutex.RLock()
	defer s.configMutex.RUnlock()
	return o.Decrypt(key)
}

func (s *Server) handleValidateNonce(r *http.Request, t api.Request, key *jose.JSONWebKey) (int, error) {
	byt, err := s.handlePut(r, key)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Invalid Request: %w", err)
	}

	if err := t.Unmarshal(byt); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Couldn't unmarshal: %w", err)
	}

	if err := s.validateNonce(t.Nonce()); err != nil {
		return http.StatusForbidden, fmt.Errorf("Invalid Request: %w", err)
	}

	return http.StatusOK, nil
}
