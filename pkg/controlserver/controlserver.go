package controlserver

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/erikh/border/pkg/config"
	"github.com/go-jose/go-jose/v3"
)

const (
	nonceSize       = 128
	nonceExpiration = 30 * time.Second
)

type Server struct {
	server     *http.Server
	listenSpec string
	config     config.Config

	nonces     map[string]time.Time
	nonceMutex sync.RWMutex

	cancelSupervision context.CancelFunc
}

// Start the control server in the background.
//
// We assume after a second, the server has started. I can't see a better way
// to do this with net/http since there is not a notifier callback. Would
// prefer a better way to launch the server without blocking.
func Start(config config.Config, listenSpec string) (*Server, error) {
	errChan := make(chan error, 1)

	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{config: config, listenSpec: listenSpec, cancelSupervision: cancel}

	s := &http.Server{
		Addr:    listenSpec,
		Handler: server.configureMux(),
	}

	go server.expireNonces(ctx)
	go func() {
		errChan <- s.ListenAndServe()
	}()

	server.server = s

	select {
	case err := <-errChan:
		return nil, err
	case <-time.After(time.Second):
		return server, nil
	}
}

// Shutdown the server. Accept a context for timing out the shutdown process.
func (s *Server) Shutdown(ctx context.Context) error {
	// idea of the defer is to cancel supervision after shutdown, to avoid a network race
	defer s.cancelSupervision()
	return s.Shutdown(ctx)
}

func (s *Server) expireNonces(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(nonceExpiration)
		}

		for n, t := range s.nonces {
			s.nonceMutex.RLock()
			if t.Before(time.Now().Add(-nonceExpiration)) {
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
	return jose.NewEncrypter(jose.A256GCM, jose.Recipient{Algorithm: jose.ED25519, Key: ed25519.PrivateKey(s.config.AuthKey).Public()}, nil)
}

func (s *Server) configureMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/nonce", s.handleNonce)
	mux.HandleFunc("/authcheck", s.handleAuthCheck)
	mux.HandleFunc("/register", s.handleRegister)
	return mux
}

// encrypts a nonce with the public key, derived from the private key. for
// authentication challenges, it is expected that this nonce will be repeated
// back to a request encrypted by the public key. As a result, both sides must
// hold the private key, which I'm not happy about, but I'm struggling to think
// of a better way.
func (s *Server) handleNonce(w http.ResponseWriter, r *http.Request) {
	byt := make([]byte, nonceSize)

	if n, err := rand.Read(byt); err != nil || n != nonceSize {
		http.Error(w, fmt.Sprintf("Invalid entropy read (size: %d, error: %v)", n, err), http.StatusInternalServerError)
		return
	}

	ok := true
	var nonce string

	// XXX potential to infinite loop; just seems really unlikely.
	for ok {
		nonce = base64.URLEncoding.EncodeToString(byt)
		s.nonceMutex.RLock()
		_, ok = s.nonces[nonce]
		s.nonceMutex.RUnlock()
	}

	s.nonceMutex.Lock()
	s.nonces[nonce] = time.Now()
	s.nonceMutex.Unlock()

	e, err := s.getEncrypter()
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not initialize encrypter: %v", err), http.StatusInternalServerError)
		return
	}

	cipherText, err := e.Encrypt([]byte(nonce))
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not encrypt ciphertext: %v", err), http.StatusInternalServerError)
		return
	}

	serialized, err := cipherText.CompactSerialize()
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not serialize JWE: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(serialized))
}

func (s *Server) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	byt, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error in body read: %v", err), http.StatusInternalServerError)
		return
	}

	o, err := jose.ParseEncrypted(string(byt))
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not parse JWE request: %v", err), http.StatusInternalServerError)
		return
	}

	nonce, err := o.Decrypt(ed25519.PrivateKey(s.config.AuthKey))
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not decrypt JWE request: %v", err), http.StatusInternalServerError)
		return
	}

	s.nonceMutex.RLock()
	t, ok := s.nonces[string(nonce)]
	s.nonceMutex.RUnlock()
	if !ok {
		http.Error(w, fmt.Sprintf("Nonce provided does not exist: %v", err), http.StatusForbidden)
		return
	}

	if t.Before(time.Now().Add(-nonceExpiration)) {
		http.Error(w, "Nonce has expired", http.StatusForbidden)
		return
	}

	s.nonceMutex.Lock()
	delete(s.nonces, string(nonce))
	s.nonceMutex.Unlock()

	// Authenticated!
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
}
