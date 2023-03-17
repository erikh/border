package config

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/erikh/border/pkg/dnsconfig"
	"github.com/erikh/border/pkg/lb"
)

type Record struct {
	Type         string           `json:"type"`
	Name         string           `json:"name"`
	LiteralValue map[string]any   `json:"value"`
	Value        dnsconfig.Record `json:"-"`
}

func trimDot(key string) string {
	if strings.HasSuffix(key, ".") {
		key = strings.TrimSuffix(key, ".")
	}

	return key
}

func addDot(key string) string {
	if !strings.HasSuffix(key, ".") {
		key += "."
	}

	return key
}

// Trim the trailing dot from zone records. Used in saving the configuration.
func (c *Config) trimZones() {
	newZones := map[string]*Zone{}

	for key, zone := range c.Zones {
		zone.SOA.Domain = trimDot(zone.SOA.Domain)
		zone.SOA.Admin = trimDot(zone.SOA.Admin)

		newServers := []string{}

		for _, server := range zone.NS.Servers {
			newServers = append(newServers, trimDot(server))
		}

		zone.NS.Servers = newServers

		for _, record := range zone.Records {
			record.Name = trimDot(record.Name)
		}

		newZones[trimDot(key)] = zone
	}

	c.Zones = newZones
}

// Decorate zones with a trailing dot. Used in loading the configuration.
func (c *Config) decorateZones() {
	newZones := map[string]*Zone{}

	for key, zone := range c.Zones {
		zone.SOA.Domain = addDot(zone.SOA.Domain)
		zone.SOA.Admin = addDot(zone.SOA.Admin)

		newServers := []string{}

		for _, server := range zone.NS.Servers {
			newServers = append(newServers, addDot(server))
		}

		zone.NS.Servers = newServers

		for _, record := range zone.Records {
			record.Name = addDot(record.Name)
		}

		newZones[addDot(key)] = zone
	}

	c.Zones = newZones
}

// decompose the "literal value" into a dnsconfig struct record, to be
// converted later into all sorts of useful stuff.
//
// this could probably be done much better with struct tags; I'm just too lazy
// at this point.
func (c *Config) convertLiterals() error {
	for _, z := range c.Zones {
		if z.NS.TTL == 0 {
			z.NS.TTL = z.SOA.MinTTL
		}

		for _, r := range z.Records {
			switch r.Type {
			case dnsconfig.TypeA:
				orig, ok := r.LiteralValue["addresses"]
				if !ok || len(orig.([]any)) == 0 {
					return fmt.Errorf("No addresses for %q A record", r.Name)
				}

				addresses := []net.IP{}

				for _, addr := range r.LiteralValue["addresses"].([]any) {
					addresses = append(addresses, net.ParseIP(addr.(string)))
				}

				var ttl uint32
				origTTL, ok := r.LiteralValue["ttl"]

				if ok {
					ttl = uint32(origTTL.(float64))
				} else {
					ttl = z.SOA.MinTTL
				}

				r.Value = &dnsconfig.A{
					Addresses: addresses,
					TTL:       ttl,
				}
			case dnsconfig.TypeLB:
				listeners := []string{}

				for _, listener := range r.LiteralValue["listeners"].([]any) {
					host, port, err := net.SplitHostPort(listener.(string))
					if err != nil {
						return fmt.Errorf("Could not parse listener %q: %v", listener, err)
					}

					peer, ok := c.Peers[host]
					if !ok {
						return fmt.Errorf("Peer %q does not exist in peers list", host)
					}

					listeners = append(listeners, net.JoinHostPort(peer.IP.String(), port))
				}

				kind, ok := r.LiteralValue["kind"].(string)
				if !ok {
					return errors.New("kind was not specified in LB record")
				}

				switch kind {
				case lb.BalanceTCP:
				default:
					return fmt.Errorf("Kind was invalid: %q", kind)
				}

				backends := []string{}

				tmp, ok := r.LiteralValue["backends"].([]any)
				if !ok {
					return errors.New("lb backends were unspecified or invalid")
				}

				for _, t := range tmp {
					backends = append(backends, t.(string))
				}

				timeout, ok := r.LiteralValue["connection_timeout"].(time.Duration)
				if !ok {
					timeout = 0
				}

				sc, ok := r.LiteralValue["simultaneous_connections"].(int)
				if !ok {
					sc = dnsconfig.DefaultSimultaneousConnections
				}

				mc, ok := r.LiteralValue["max_connections_per_address"].(int)
				if !ok {
					mc = dnsconfig.DefaultMaxConnectionsPerAddress
				}

				ttl, ok := r.LiteralValue["ttl"].(uint32)
				if !ok {
					ttl = z.SOA.MinTTL
				}

				r.Value = &dnsconfig.LB{
					Listeners:                listeners,
					Kind:                     kind,
					Backends:                 backends,
					SimultaneousConnections:  sc,
					MaxConnectionsPerAddress: mc,
					ConnectionTimeout:        timeout,
					TTL:                      ttl,
				}
			default:
				return fmt.Errorf("invalid type for record %q", r.Name)
			}
		}
	}

	return nil
}
