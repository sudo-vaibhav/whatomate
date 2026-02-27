# Coolify Service Template for Whatomate

This directory contains the Coolify service template for deploying Whatomate.

## Files

```
coolify/
├── svgs/
│   └── whatomate.svg          # Service logo
├── templates/
│   └── compose/
│       └── whatomate.yaml     # Service template
└── README.md                  # This file
```

## Template Features

- **PostgreSQL 17** - Auto-configured database with generated credentials
- **Redis 7** - With LRU eviction and 256MB memory limit
- **Auto-generated secrets**:
  - `SERVICE_BASE64_64_ENCRYPTION` → AES-256 encryption key
  - `SERVICE_BASE64_64_JWT` → JWT signing secret
  - `SERVICE_USER_POSTGRES` / `SERVICE_PASSWORD_POSTGRES` → DB credentials
- **Required configuration**: `ADMIN_PASSWORD` (user must set during deployment)
- **Production defaults**: Rate limiting enabled, trust_proxy=true

## Submitting to Coolify

1. Fork [coolify](https://github.com/coollabsio/coolify)
2. Copy files:
   - `svgs/whatomate.svg` → `svgs/whatomate.svg`
   - `templates/compose/whatomate.yaml` → `templates/compose/whatomate.yaml`
3. Open a PR targeting the `next` branch
4. Reference: [Adding a new service template](https://coolify.io/docs/get-started/contribute/service)

## Testing Locally

Test the template using Coolify's "Docker Compose Empty" deployment:

1. In Coolify, create a new service → "Docker Compose Empty"
2. Paste the contents of `whatomate.yaml`
3. Set required environment variable: `ADMIN_PASSWORD`
4. Deploy and verify at `http://<your-domain>`

## Default Login

After deployment, login with:
- Email: `ADMIN_EMAIL` (default: `admin@example.com`)
- Password: `ADMIN_PASSWORD` (as configured)
