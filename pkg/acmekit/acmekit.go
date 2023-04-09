package acmekit

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"

	"github.com/mholt/acmez"
	"github.com/mholt/acmez/acme"
	"go.uber.org/zap"
)

type Certificate struct {
	Chain      []byte `json:"chain"`
	PrivateKey []byte `json:"private_key"`
}

type Account struct {
	Information  acme.Account            `json:"info"`
	PrivateKey   []byte                  `json:"private_key"`
	Certificates map[string]*Certificate `json:"certificates"`
}

type ACMEParams struct {
	Account      *Account `json:"account"`
	IgnoreVerify bool     `json:"ignore_verify"`
	Directory    *url.URL `json:"acme_directory"`
	ContactInfo  []string `json:"contact_info"`
}

func getZapLogger() (*zap.Logger, error) {
	return zap.NewDevelopment()
}

func generatePrivateKey() (*ecdsa.PrivateKey, []byte, error) {
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not generate private key for ACME after generation: %w", err)
	}

	marshalledPKey, err := x509.MarshalECPrivateKey(pkey)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not marshal private key for ACME after generation: %w", err)
	}

	return pkey, marshalledPKey, nil
}

// Determine if the user has an already properly registered and active account
// with the ACME server.
func (ap *ACMEParams) HasValidAccount(ctx context.Context) (bool, error) {
	if ap.Account != nil && ap.Account.Information.Status == "valid" {
		client, err := ap.makeClient()
		if err != nil {
			return false, err
		}

		account, err := client.GetAccount(ctx, ap.Account.Information)
		if err != nil {
			return false, fmt.Errorf("Error looking up ACME account on remote server: %w", err)
		}

		ap.Account.Information = account
		return ap.Account.Information.Status == "valid", nil
	}

	return false, nil
}

// makeClient makes an ACME client
func (ap *ACMEParams) makeClient() (*acmez.Client, error) {
	logger, err := getZapLogger()
	if err != nil {
		return nil, fmt.Errorf("Could not create logger: %w", err)
	}

	return &acmez.Client{
		Client: &acme.Client{
			Directory: ap.Directory.String(),
			HTTPClient: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: ap.IgnoreVerify,
					},
				},
			},
			Logger: logger,
		},
	}, nil
}

// CreateAccount creates a private key, attempts to create an account with the
// ACME server, and returns no error if that was successful, filling the
// properties in the Account member if so. If the Account field is already
// populated, it will overwrite those values with new ones. Use HasValidAccount
// to determine if the account is already set.
func (ap *ACMEParams) CreateAccount(ctx context.Context) error {
	pkey, marshalledPKey, err := generatePrivateKey()
	if err != nil {
		return err
	}

	account := acme.Account{
		Contact:              ap.ContactInfo,
		TermsOfServiceAgreed: true,
		PrivateKey:           pkey,
	}

	client, err := ap.makeClient()
	if err != nil {
		return fmt.Errorf("Error while making ACME client: %w", err)
	}

	account, err = client.NewAccount(ctx, account)
	if err != nil {
		return fmt.Errorf("Could not create new ACME account: %w", err)
	}

	ap.Account = &Account{
		Information: account,
		PrivateKey:  marshalledPKey,
	}

	return nil
}

// GetCertificate generates a private key, gets a client, and attempts to
// obtain a certificate. It will overwrite any existing certificate. If there
// is no account, it will fail.
func (ap *ACMEParams) GetCertificate(ctx context.Context, domain string) error {
	valid, err := ap.HasValidAccount(ctx)
	if err != nil {
		return err
	}

	if !valid {
		return fmt.Errorf("ACME Account is invalid, please create a new account")
	}

	pkey, marshalledPKey, err := generatePrivateKey()
	if err != nil {
		return err
	}

	client, err := ap.makeClient()
	if err != nil {
		return fmt.Errorf("Error while making ACME client: %w", err)
	}

	certs, err := client.ObtainCertificate(ctx, ap.Account.Information, pkey, []string{domain})
	if err != nil {
		return fmt.Errorf("Error obtaining certificate from ACME directory: %w", err)
	}

	if ap.Account.Certificates == nil {
		ap.Account.Certificates = map[string]*Certificate{}
	}

	ap.Account.Certificates[domain] = &Certificate{
		Chain:      certs[0].ChainPEM,
		PrivateKey: marshalledPKey,
	}

	return nil
}
