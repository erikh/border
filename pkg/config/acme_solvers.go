package config

import (
	"context"

	"github.com/mholt/acmez/acme"
)

type ACMEDNSSolver struct {
	domain string
	config *Config
}

func (c *Config) DNSSolver(domain string) *ACMEDNSSolver {
	return &ACMEDNSSolver{domain: domain, config: c}
}

func (dns *ACMEDNSSolver) Present(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (dns *ACMEDNSSolver) CleanUp(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (dns *ACMEDNSSolver) Wait(ctx context.Context, chal acme.Challenge) error {
	return nil
}

type ACMEALPNSolver struct {
	domain string
	config *Config
}

func (c *Config) ALPNSolver(domain string) *ACMEALPNSolver {
	return &ACMEALPNSolver{domain: domain, config: c}
}

func (dns *ACMEALPNSolver) Present(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (dns *ACMEALPNSolver) CleanUp(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (dns *ACMEALPNSolver) Wait(ctx context.Context, chal acme.Challenge) error {
	return nil
}

type ACMEHTTPSolver struct {
	domain string
	config *Config
}

func (c *Config) HTTPSolver(domain string) *ACMEHTTPSolver {
	return &ACMEHTTPSolver{domain: domain, config: c}
}

func (dns *ACMEHTTPSolver) Present(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (dns *ACMEHTTPSolver) CleanUp(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (dns *ACMEHTTPSolver) Wait(ctx context.Context, chal acme.Challenge) error {
	return nil
}
