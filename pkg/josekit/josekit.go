package josekit

import (
	"crypto/rand"

	"github.com/go-jose/go-jose/v3"
)

// MakeKey makes an AES-backed JWK
func MakeKey(kid string) (*jose.JSONWebKey, error) {
	key := make([]byte, 32)

	if _, err := rand.Read(key); err != nil {
		return nil, err
	}

	return &jose.JSONWebKey{Key: key, KeyID: kid, Algorithm: string(jose.A256KW)}, nil
}
