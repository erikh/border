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

func getNonce(server *Server) (*http.Response, error) {
	url := fmt.Sprintf("http://%s/nonce", server.listener.Addr())
	return http.Get(url)
}

func authCheck(server *Server, body io.Reader) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req, err := http.NewRequest("PUT", fmt.Sprintf("http://%s/authcheck", server.listener.Addr()), body)
	if err != nil {
		return nil, err
	}

	return http.DefaultClient.Do(req.WithContext(ctx))
}

func TestStartupShutdown(t *testing.T) {
	server, err := Start(makeConfig(t), ":0", 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := getNonce(server)
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

	if _, err := getNonce(server); err == nil {
		t.Fatal("Server is still up; should no longer be")
	}
}

func TestNonce(t *testing.T) {
	server, err := Start(makeConfig(t), ":0", 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})

	resp, err := getNonce(server)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("Nonce check failed: status code was not 200, was %d: error: %v", resp.StatusCode, string(out))
	}

	defer resp.Body.Close()

	resp, err = authCheck(server, resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("Nonce check failed: status code was not 200, was %d: error: %v", resp.StatusCode, string(out))
	}

	resp.Body.Close()

	// fetch a nonce, wait for it to time out, and try again. Should fail.
	resp, err = getNonce(server)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("Nonce check failed: status code was not 200, was %d: error: %v", resp.StatusCode, string(out))
	}

	defer resp.Body.Close()

	time.Sleep(20 * time.Millisecond) // nonce should be expired or already pruned now

	resp, err = authCheck(server, resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	// resp should not have a 200 status here. Should be 403.
	if resp.StatusCode != http.StatusForbidden {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("Auth check did not fail: status code was not 403, was %d: error: %v", resp.StatusCode, string(out))
	}

	resp.Body.Close()
}
