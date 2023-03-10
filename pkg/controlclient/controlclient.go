package controlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/josekit"
	"github.com/go-jose/go-jose/v3"
)

var (
	ErrAcquireNonce = errors.New("Could not acquire nonce")
	ErrMarshal      = errors.New("Could not marshal payload")
	ErrBadResponse  = errors.New("Invalid response from API server")
	ErrDecrypt      = errors.New("Could not decrypt response")
	ErrEncrypt      = errors.New("Could not encrypt payload")
)

type Client struct {
	AuthKey *jose.JSONWebKey `json:"key"`
	BaseURL *url.URL         `json:"base_url"`
}

func (c *Client) Exchange(endpoint string, msg api.Message, res api.Message) error {
	u := c.BaseURL.JoinPath("/nonce")
	resp, err := http.Get(u.String())
	if err != nil {
		return errors.Join(ErrAcquireNonce, err)
	}

	if resp.StatusCode != http.StatusOK {
		byt, err := io.ReadAll(resp.Body)
		if err != nil {
			return errors.Join(ErrAcquireNonce, err)
		}

		return fmt.Errorf("Status was not OK after nonce call, status was %v: %v", resp.StatusCode, string(byt))
	}

	nonce, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Join(ErrAcquireNonce, err)
	}

	enc, err := jose.ParseEncrypted(string(nonce))
	if err != nil {
		return errors.Join(ErrDecrypt, err)
	}

	nonce, err = enc.Decrypt(c.AuthKey)
	if err != nil {
		return errors.Join(ErrDecrypt, err)
	}

	msg.SetNonce(nonce)

	encrypter, err := josekit.GetEncrypter(c.AuthKey)
	if err != nil {
		return errors.Join(ErrEncrypt, err)
	}

	byt, err := msg.Marshal()
	if err != nil {
		return errors.Join(ErrMarshal, err)
	}

	cipherText, err := encrypter.Encrypt(byt)
	if err != nil {
		return errors.Join(ErrEncrypt, err)
	}

	out, err := cipherText.CompactSerialize()
	if err != nil {
		return errors.Join(ErrEncrypt, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	u = c.BaseURL.JoinPath(endpoint)

	req, err := http.NewRequest("PUT", u.String(), bytes.NewBuffer([]byte(out)))
	if err != nil {
		return err
	}

	resp, err = http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return errors.Join(ErrBadResponse, err)
	}

	if resp.StatusCode != http.StatusOK {
		byt, err := io.ReadAll(resp.Body)
		if err != nil {
			return errors.Join(ErrBadResponse, err)
		}

		return fmt.Errorf("Status was not OK after %T call, status was %v: %v", msg, resp.StatusCode, string(byt))
	}

	byt, err = io.ReadAll(resp.Body)
	if err != nil {
		return errors.Join(ErrBadResponse, err)
	}

	enc, err = jose.ParseEncrypted(string(byt))
	if err != nil {
		return errors.Join(ErrDecrypt, err)
	}

	byt, err = enc.Decrypt(c.AuthKey)
	if err != nil {
		return errors.Join(ErrDecrypt, err)
	}

	if err := json.Unmarshal(byt, res); err != nil {
		return errors.Join(ErrBadResponse, err)
	}

	return nil
}