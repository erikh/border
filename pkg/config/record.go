package config

import (
	"fmt"
	"strings"

	"github.com/erikh/border/pkg/dnsconfig"
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
				a := &dnsconfig.A{}
				a.TTL = z.SOA.MinTTL

				r.Value = a
			case dnsconfig.TypeLB:
				lb := &dnsconfig.LB{}
				lb.TTL = z.SOA.MinTTL
				lb.MaxConnectionsPerAddress = dnsconfig.DefaultMaxConnectionsPerAddress
				lb.SimultaneousConnections = dnsconfig.DefaultSimultaneousConnections

				r.Value = lb
			default:
				return fmt.Errorf("invalid type for record %q", r.Name)
			}

			if err := r.parseLiteral(); err != nil {
				return fmt.Errorf("Error parsing record %q: %v", r.Name, err)
			}
		}
	}

	return nil
}
