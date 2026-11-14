# lego DNS provider catalogue

`catalogue.json` is generated from the provider registry and TOML documentation in
the `github.com/go-acme/lego/v4` version pinned in `go.mod`. Documentation remains
under lego's MIT license, reproduced in `LEGO-LICENSE`.

Regenerate with `go run ./scripts/generate-dns-providers`. CI/review can check for
drift with `go run ./scripts/generate-dns-providers -check`.

Every registered DNS provider and alias is represented. `manual` is a terminal
solver, not a DNS service, and is explicitly unavailable for unattended web orders.
The external-program solver is supported and requires a server-installed program.
Alternative authentication modes, SDK identity chains, file paths and tuning
options are described by the version-matched upstream documentation. Do not mark
every credential field required: many providers offer mutually exclusive modes.

DNS-01 needs outbound access to the CA/DNS API and DNS resolution, but no inbound
80/443 listener. HTTP-01 is an explicit alternative for public port 80 deployments.
