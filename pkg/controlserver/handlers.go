package controlserver

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/erikh/border/pkg/api"
	"github.com/go-jose/go-jose/v3"
)

// encrypts a nonce with the key. for authentication challenges, it is expected
// that this nonce will be repeated back to a request.
func (s *Server) handleNonce(w http.ResponseWriter, r *http.Request) {
	byt := make([]byte, nonceSize)

	ok := true
	var nonce string

	// XXX potential to infinite loop; just seems really unlikely.
	for ok {
		if n, err := rand.Read(byt); err != nil || n != nonceSize {
			http.Error(w, fmt.Sprintf("Invalid entropy read (size: %d, error: %v)", n, err), http.StatusInternalServerError)
			return
		}

		nonce = string(byt)

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

// handlePut deserializes a JWE request, which should be serviced via the PUT HTTP verb.
func (s *Server) handlePut(r *http.Request) ([]byte, error) {
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
	return o.Decrypt(s.config.AuthKey)
}

func (s *Server) handleValidateNonce(r *http.Request, t api.NonceRequired) (int, error) {
	byt, err := s.handlePut(r)
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

func (s *Server) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	nonce := make(api.AuthCheck, nonceSize)

	if code, err := s.handleValidateNonce(r, nonce); err != nil {
		http.Error(w, fmt.Sprintf("Nonce validation failed: %v", err), code)
		return
	}

	// Authenticated!
}

func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var c api.ConfigUpdateRequest

	if code, err := s.handleValidateNonce(r, &c); err != nil {
		http.Error(w, fmt.Sprintf("Nonce validation failed: %v", err), code)
		return
	}

	s.configMutex.Lock()
	oldConfig := s.config
	s.config = c.Config
	// XXX hack around the lack of JSON serialization for FilenamePrefix
	s.config.FilenamePrefix = oldConfig.FilenamePrefix
	s.configMutex.Unlock()

	s.saveConfig(w)
}

func (s *Server) handlePeerRegister(w http.ResponseWriter, r *http.Request) {
	var peerRequest api.PeerRegistrationRequest

	if code, err := s.handleValidateNonce(r, &peerRequest); err != nil {
		http.Error(w, fmt.Sprintf("Nonce validation failed: %v", err), code)
		return
	}

	s.configMutex.Lock()
	s.config.Peers = append(s.config.Peers, peerRequest.Peer)
	s.configMutex.Unlock()

	s.saveConfig(w)
}
