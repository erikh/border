package controlserver

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/erikh/border/pkg/config"
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

	return o.Decrypt(s.config.AuthKey)
}

type NonceRequired interface {
	Unmarshal([]byte) error
	Nonce() string
}

type AuthCheck []byte

func (ac AuthCheck) Unmarshal(byt []byte) error {
	copy(ac, byt)
	return nil
}

func (ac AuthCheck) Nonce() string {
	return string(ac)
}

func (s *Server) handleValidateNonce(r *http.Request, t NonceRequired) (int, error) {
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
	nonce := make(AuthCheck, nonceSize)

	if code, err := s.handleValidateNonce(r, nonce); err != nil {
		http.Error(w, fmt.Sprintf("Nonce validation failed: %v", err), code)
		return
	}

	// Authenticated!
}

type ConfigUpdateRequest struct {
	NonceValue []byte        `json:"nonce"`
	Config     config.Config `json:"config"`
}

func (cur *ConfigUpdateRequest) Unmarshal(byt []byte) error {
	return json.Unmarshal(byt, cur)
}

func (cur *ConfigUpdateRequest) Nonce() string {
	return string(cur.NonceValue)
}

func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var config ConfigUpdateRequest

	if code, err := s.handleValidateNonce(r, &config); err != nil {
		http.Error(w, fmt.Sprintf("Nonce validation failed: %v", err), code)
		return
	}

	s.config = config.Config
	// FIXME marshal to disk
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
}
