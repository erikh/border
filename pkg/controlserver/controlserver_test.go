package controlserver

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/erikh/border/pkg/config"
	"github.com/go-jose/go-jose/v3"
)

func makeConfig(t *testing.T) config.Config {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	return config.Config{AuthKey: &jose.JSONWebKey{Key: priv, KeyID: "test", Algorithm: string(jose.ECDH_ES_A256KW)}}
}

func TestStartupShutdown(t *testing.T) {
	server, err := Start(makeConfig(t), ":0")
	if err != nil {
		t.Fatal(err)
	}

	url := fmt.Sprintf("http://%s/nonce", server.listener.Addr())
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("Nonce check failed: status code was not 200, was %d: error: %v", resp.StatusCode, string(out))
	}

	resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}

	url = fmt.Sprintf("http://%s/nonce", server.listener.Addr())
	if _, err := http.Get(url); err == nil {
		t.Fatal("Server is still up; should no longer be")
	}
}

func TestNonce(t *testing.T) {
}
