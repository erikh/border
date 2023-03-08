package controlserver

import (
	"crypto/rand"
	"encoding/base64"
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

func (s *Server) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	nonce, err := s.handlePut(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid Request: %v", err), http.StatusInternalServerError)
		return
	}

	if err := s.validateNonce(string(nonce)); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusForbidden)
		return
	}

	// Authenticated!
}

type ConfigUpdateRequest struct {
	Nonce  string        `json:"nonce"`
	Config config.Config `json:"config"`
}

func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Invalid HTTP Method for Request", http.StatusMethodNotAllowed)
		return
	}
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
}
