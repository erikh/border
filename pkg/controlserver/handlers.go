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
	byt := make([]byte, NonceSize)

	ok := true
	var nonce string

	// XXX potential to infinite loop; just seems really unlikely.
	for ok {
		if n, err := rand.Read(byt); err != nil || n != NonceSize {
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

	serialized, err := api.EncryptResponse(s.config.AuthKey, api.AuthCheck(nonce))
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not serialize JWE: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write(serialized) // nolint:errcheck
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

func (s *Server) handleValidateNonce(r *http.Request, t api.Request) (int, error) {
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
	nonce := make(api.AuthCheck, NonceSize)

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

	serialized, err := api.EncryptResponse(s.config.AuthKey, api.NilResponse{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Error encrypting response: %v", err), http.StatusInternalServerError)
		return
	}
	// if we do not do this after encryption, the authkey may change, which will gum up the
	// encryption of the response. Since there is no response this is not an
	// issue in theory, but in practice the encryption will break, rendering the
	// response invalid.
	s.configMutex.Lock()
	oldConfig := s.config
	s.config = c.Config
	// XXX hack around the lack of JSON serialization for FilenamePrefix
	s.config.FilenamePrefix = oldConfig.FilenamePrefix
	s.configMutex.Unlock()
	s.saveConfig(w)

	w.Write(serialized) // nolint:errcheck
}

func (s *Server) handlePeerRegister(w http.ResponseWriter, r *http.Request) {
	var peerRequest api.PeerRegistrationRequest

	if code, err := s.handleValidateNonce(r, &peerRequest); err != nil {
		http.Error(w, fmt.Sprintf("Nonce validation failed: %v", err), code)
		return
	}

	s.configMutex.Lock()
	s.config.Peers[peerRequest.Name] = peerRequest.Peer
	s.configMutex.Unlock()

	s.saveConfig(w)

	serialized, err := api.EncryptResponse(s.config.AuthKey, api.NilResponse{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Error encrypting response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(serialized)) // nolint:errcheck
}

func (s *Server) handleConfigReload(w http.ResponseWriter, r *http.Request) {
	var reloadRequest api.ConfigReloadRequest

	if code, err := s.handleValidateNonce(r, &reloadRequest); err != nil {
		http.Error(w, fmt.Sprintf("Nonce validation failed: %v", err), code)
		return
	}

	if err := s.config.Reload(); err != nil {
		http.Error(w, fmt.Sprintf("Error reloading configuration: %v", err), http.StatusInternalServerError)
		return
	}

	serialized, err := api.EncryptResponse(s.config.AuthKey, api.NilResponse{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Error encrypting response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(serialized)) // nolint:errcheck
}

func (s *Server) handleElection(w http.ResponseWriter, r *http.Request) {
}
