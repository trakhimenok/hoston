# hoston

> — Heuston, we have a domain!
> — Roger that, we need the hoston — it's solid as Boston.

Domain & hosting setup CLI. Automates DNS configuration, hosting setup, and custom domain provisioning across NameCheap, CloudFlare, Firebase, and GitHub Pages.

## Install

```sh
go install github.com/trakhimenok/hoston@latest
```

Or build from source:

```sh
git clone https://github.com/trakhimenok/hoston.git
cd hoston
go build -o hoston .
```

## Prerequisites

1. **NameCheap API access**: Enable at https://ap.www.namecheap.com/settings/tools/apiaccess/ and whitelist your IP
2. **CloudFlare API token**: Create a Custom Token at https://dash.cloudflare.com/profile/api-tokens with `Zone > Zone: Edit` and `Zone > DNS: Edit` permissions (the "Edit zone DNS" template lacks zone creation rights)
3. **Firebase tools** (for Firebase hosting): `npm i -g firebase-tools && firebase login`
4. **GitHub CLI** (for GitHub Pages): `brew install gh && gh auth login`

## Quick Start

```sh
# Store credentials (one-time)
hoston auth namecheap
hoston auth cloudflare

# Set up a domain
hoston setup example.com
```

## Commands

### `hoston auth <provider>`

Store API credentials securely in macOS Keychain.

```sh
hoston auth namecheap    # NameCheap username & API key
hoston auth cloudflare   # CloudFlare API token
```

### `hoston setup <domain>`

Interactive wizard that:

1. Verifies credentials for NameCheap + CloudFlare
2. Adds domain to CloudFlare
3. Updates NameCheap nameservers → CloudFlare
4. Waits for DNS propagation
5. Sets up hosting (Firebase or GitHub Pages)
6. Configures custom domain
7. Creates DNS records
8. Validates HTTPS and site availability

If any step fails, it shows manual instructions and waits for confirmation.

### `hoston status <domain>`

Check domain status across providers. *(Coming soon)*

## Architecture

```
hoston
├── cmd/                     # CLI commands (cobra)
├── internal/
│   ├── cloudflare/          # CloudFlare API (cloudflare-go SDK)
│   ├── namecheap/           # NameCheap API (XML/HTTP)
│   ├── firebase/            # Firebase CLI wrapper
│   ├── github/              # GitHub Pages via gh CLI
│   ├── keychain/            # macOS Keychain (go-keychain)
│   ├── dns/                 # DNS validation helpers
│   └── wizard/              # Setup wizard orchestration
├── main.go
└── go.mod
```

## License

MIT
