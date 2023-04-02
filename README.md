# Border: A modern approach to DNS & Load Balancers

Border is a combination of DNS service and Load Balancer, It uses this
combination, internal health checking, and primitive consensus algorithms to
automatically heal your network when there are issues. If you lose a border
node, one can be immediately added to the cluster without additional
configuration. Zones and Balancing rules will automatically propagate to the
new node.

All network resources are treated like DNS entries, and DNS propagation ("Zone
Transfers") do not use the primitive and insecure AXFR protocol. They use
[JOSE](https://datatracker.ietf.org/wg/jose/documents/) to distribute full
configuration of the server via modern protocols with modern encryption and
modern amenities. All nodes subscribe to a publisher node, which is in "legacy"
terms the "master" or "owner" of the network. The publisher is responsible for
distributing configuration to the other nodes.

Border does not depend on a consensus database. It holds its own elections and
it is strongly recommended for this reason you run an even number of nodes in a
production configuration, so that when a service goes down, an election can be
held and it will be guaranteed that one will win eventually.

## Rationale

Why do this?

In typical scenarios, you have one of a few configurations that all have their
own problems:

- In many cases, you have fixed IPs provided via a very rigidly deployed DNS
  service. Virtual IPs, or technologies like BGP and Anycast are employed to
  retain the value of these IP addresses. DNS is, in this case, a very brittle
  lynchpin providing static records to the network which when modified, tend to
  cause chaos as it is a major event.

- Other cases leverage a round-robin distribution of A records with a very
  short TTL on the DNS server. This increases load to the DNS service, which is
  usually not an issue, but also increases the severity of a DNS outage. This
  works well with e.g. haproxy, but if the haproxy is removed prematurely to a
  DNS change, there will be forced network hiccups.

- Other, simpler configurations either stack load balancers, proxy from haproxy
  to a secondary proxying webserver, e.g., nginx, but no matter how this is
  sliced, there is a single point of failure: the load balancer, but usually
  DNS is also a notable point of failure in the event the host running the load
  balancer is lost.

Additionally, third party health monitoring must be employed for all these
scenarios. The beauty of Border is that it monitors itself through the
conjunction of both protocols combined with consensus.

Border cannot eliminate all failures. TCP is still TCP and when the connection
is gone, TCP must do something about that, which is usually to simply fail the
connection. Border, however, tries to eliminate the administrative overhead of
such an event.

## Features

Border is trying to pack a lot of features and not just be a simple tool. We
are dedicated to bringing a richer experience to fronting websites for
administrators.

- [x] Provides TCP and HTTP Load Balancing
- [ ] TLS Termination
- [ ] Load Balancing of less typical DNS situations, such as SRV records (think
      tools like Samba or LDAP).
- [x] Health checks are a part of DNS, and when a health check is failed, DNS
      is automatically adjusted.
- [x] Zone Transfers do not use the unwieldy and frequently insecure AXFR
      protocol, instead opting for the protections provided by JOSE. Full
      configuration is synced, not just zones.
- [ ] Built-in Let's Encrypt and ACME support
  - [ ] For TLS Termination
  - [ ] For DNSSEC (still need to look deeper into this one)
- [x] Self-Distributing architecture means fire-and-forget deployments, and a
      fully STONITH (Shoot the offending node in the head) architecture.
- [ ] ngrok-like agent to help border traverse NAT firewalls as well as more
      entrenched network configurations behind e.g. Corporate Firewalls.
- [ ] Split Horizon support baked into the service, on a per network and per zone basis.
- [ ] Capacity management in the config, e.g., "this webserver can handle 10k
      connections at a time, and that one can handle 5k, so don't route more than
      that there".
  - [ ] Consensus based connection tracking so that load balancers can manage
        all servers in a criss-cut pattern, not just pool of servers that are lost
        when the balancer goes down.
- [ ] We are debating adding a caching proxy ala Squid / Varnish, as it may
      also be a good fit in this service with a minimal footprint overall.

## Some operational notes

[Here](example.yaml) is an example configuration. Documentation will come soon,
but this displays most of the service's features at this time.

Elections are held when the publisher is no longer responsive. This is a
configurable parameter (at least, eventually). At the point an election is
held, all members vote for the service with what they think has the highest
uptime. This should result in a clear winner as if the publisher has been
working up to this point, this information should be communicated in the
configuration.

The appropriate cadence for replacing a failing or terminated load balancer is
such:

- Termination event of original load balancer
- Wait for health checks to fail, and any necessary elections to complete. This
  takes approximately a second.
- Create a new peer in the configuration, and send it `border client
updateconfig <myconfigfile>` to the publisher. You can use `border client
identifypublisher` to determine the publisher.
- Start the new peer with the updated configuration.

Please note that between the termination point and the raise of the new
instance, that unless you have lost _all_ border nodes, health checking will
automatically heal the affected load balancing and DNS records pointing at the
failed instance. This works _today_.

Load Balancers are configured like a DNS record. The A records for a website
are maintained as records pointing to border. Contrast with ALIAS records on
Amazon Web Services' Route 53 and Elastic Load Balancer. Each one will have
configurable parameters similar to a SOA record's notion of TTL and cache.

Border agents (tunneling proxy) will use JOSE keys to identify themselves to
the network instead of an IP address, and their DNS records will represent
that. Contrast with a SPF record's use of the TXT record type.

Border agent is a well documented and well specified protocol that can be
implemented by web service frameworks as well as web servers themselves such as
Caddy or Nginx.

Border's impact on whois distribution basically expects you to use higher TTL
records and let the DNS protocol do its job properly, allowing for fallback to
other nameservers. In the event a nameserver fails, A quorum of 4 nameservers
should keep another three alive, allowing you to adjust the whois records in the
event the host is completely lost. We feel this is a safer DR strategy than
investing a lot of infrastructure into retaining the IP addresses at all costs,
as it is much simpler to maintain.

## Status

Border is in an early alpha stage. It is functional, but lacks many of the
features you would expect from a product of its type. Encouragement, testing,
and patches (!) are strongly encouraged, but "betting the farm" on this product
at this point would be an impressive display of your own lack of wisdom.

## Author

Erik Hollensbe <erik+github@hollensbe.org>
