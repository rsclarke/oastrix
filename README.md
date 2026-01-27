# Oastrix

An Out-of-Band Application Security Testing (OAST) tool for detecting blind vulnerabilities.

## Features

- **Automatic TLS** via Let's Encrypt (ACME with DNS-01 challenges)
- HTTP/HTTPS request capture with full headers and body
- DNS query capture (UDP and TCP)
- API key authentication
- SQLite storage (no external dependencies)
- Single binary deployment

## Quick Start

### Build

```bash
go build -o oastrix ./cmd/oastrix
```

### Start the server (production)

```bash
sudo ./oastrix server --domain oastrix.example.com
```

This will:
1. Start HTTP on port 80, HTTPS on port 443, DNS on port 53
2. Automatically obtain a Let's Encrypt certificate via DNS-01 challenge
3. Print an API key on first run (save it!)

### Start the server (development)

```bash
./oastrix server --no-acme --http-port 8080 --dns-port 5354
```

### Generate a token

```bash
export OASTRIX_API_KEY="oastrix_..."
./oastrix generate --label "test"
```

Output:
```
Token: abc123xyz789

Payloads:
  http://abc123xyz789.oastrix.example.com/
  https://abc123xyz789.oastrix.example.com/
  abc123xyz789.oastrix.example.com (DNS)
```

### Check for interactions

```bash
./oastrix interactions <token>
```

Output:
```
TIME                  KIND  REMOTE            SUMMARY
2024-01-15 10:30:45   http  192.168.1.1:4532  GET /path HTTP/1.1
2024-01-15 10:30:46   dns   192.168.1.1:5353  A abc123.domain udp
```

### List all tokens

```bash
./oastrix list
```

### Delete a token

```bash
./oastrix delete <token>
```

## Configuration

### Server Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| --domain | OASTRIX_DOMAIN | localhost | Domain for token URLs |
| --http-port | OASTRIX_HTTP_PORT | 80 | HTTP capture port |
| --https-port | OASTRIX_HTTPS_PORT | 443 | HTTPS capture port |
| --api-port | OASTRIX_API_PORT | 8081 | API server port |
| --dns-port | OASTRIX_DNS_PORT | 53 | DNS server port |
| --public-ip | OASTRIX_PUBLIC_IP | - | Public IP address of the server (see below) |
| --db | OASTRIX_DB | oastrix.db | SQLite database path |
| --pepper | OASTRIX_PEPPER | (auto) | HMAC pepper for API keys |

### TLS Flags

| Flag | Default | Description |
|------|---------|-------------|
| --no-acme | false | Disable automatic TLS (HTTPS server not started) |
| --acme-email | - | Email for Let's Encrypt notifications |
| --acme-staging | false | Use Let's Encrypt staging CA |
| --tls-cert | - | Manual TLS certificate path |
| --tls-key | - | Manual TLS key path |

### TLS Modes

| Flags | Behavior |
|-------|----------|
| (default) | ACME enabled, automatic Let's Encrypt certs |
| --tls-cert + --tls-key | Manual TLS, uses provided certificates |
| --no-acme | No HTTPS server |

### Public IP

The `--public-ip` flag specifies the server's external IP address. It is used for:

1. **DNS A record responses** - When clients query `ns1.<domain>`, the DNS server returns this IP
2. **ACME DNS-01 challenges** - Let's Encrypt resolves the nameserver to validate certificate requests

This flag is **required** when ACME is enabled. Without it, Let's Encrypt cannot locate your DNS server to verify the `_acme-challenge` TXT records.

For development with `--no-acme`, this flag is optional.

### CLI Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| --api-key | OASTRIX_API_KEY | - | API key (required) |
| --api-url | OASTRIX_API_URL | http://localhost:8081 | API server URL |

## Production Deployment

### Prerequisites

1. A domain with NS records pointing to your server
2. Root access or `setcap` for binding to ports 80, 443, 53

### DNS Setup

Configure your domain's NS records to point to your oastrix server:

```
oastrix.example.com.  NS  ns1.oastrix.example.com.
ns1.oastrix.example.com.  A  <your-server-ip>
```

### Running

```bash
# With root
sudo ./oastrix server --domain oastrix.example.com --public-ip <your-server-ip> --acme-email admin@example.com

# Or with capabilities (no root)
sudo setcap cap_net_bind_service=+ep ./oastrix
./oastrix server --domain oastrix.example.com --public-ip <your-server-ip> --acme-email admin@example.com
```

### Certificate Storage

Certificates are stored in `./certmagic/` (relative to database location). This directory contains:
- ACME account key
- Certificates and private keys
- Renewal metadata

Ensure this directory persists across restarts.

## Troubleshooting

### ACME certificate fails

1. **DNS not reachable**: Ensure UDP/TCP port 53 is publicly accessible
2. **NS records wrong**: Verify `dig NS oastrix.example.com` returns your server
3. **Rate limited**: Use `--acme-staging` for testing, switch to production when ready

### Port binding fails

```
error: listen tcp :80: bind: permission denied
```

Run with `sudo` or use `setcap`:
```bash
sudo setcap cap_net_bind_service=+ep ./oastrix
```

## Security Notes

- API keys are shown only once at creation - store securely
- The database contains captured request data - secure file permissions
- Certificates directory contains private keys - restrict access (0700)
- Use `--pepper` or `OASTRIX_PEPPER` for consistent API key hashing across restarts
