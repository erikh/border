package controlserver

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-jose/go-jose/v3"
)

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
	if r.Method != "PUT" {
		http.Error(w, "Invalid HTTP Method for Request", http.StatusMethodNotAllowed)
		return
	}

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

	nonce, err := o.Decrypt(s.config.AuthKey)
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

	if t.Before(time.Now().Add(-s.expireTime)) {
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
