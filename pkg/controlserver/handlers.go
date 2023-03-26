package controlserver

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/erikh/border/pkg/api"
)

// encrypts a nonce with the key. for authentication challenges, it is expected
// that this nonce will be repeated back to a request.
func (s *Server) handleNonce(req api.Request) (api.Message, error) {
	byt := make([]byte, api.NonceSize)

	ok := true
	var nonce string

	// XXX potential to infinite loop; just seems really unlikely.
	for ok {
		if n, err := rand.Read(byt); err != nil || n != api.NonceSize {
			return nil, fmt.Errorf("Invalid entropy read (size: %d, error: %v)", n, err)
		}

		nonce = string(byt)

		s.nonceMutex.RLock()
		_, ok = s.nonces[nonce]
		s.nonceMutex.RUnlock()
	}

	s.nonceMutex.Lock()
	s.nonces[nonce] = time.Now()
	s.nonceMutex.Unlock()

	return api.AuthCheck(nonce), nil
}
