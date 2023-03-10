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

func GetEncrypter(authKey *jose.JSONWebKey) (jose.Encrypter, error) {
	return jose.NewEncrypter(jose.A256GCM, jose.Recipient{Algorithm: jose.A256KW, Key: authKey}, nil)
}
