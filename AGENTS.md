# CLAUDE.md

This file provides guidance to AI agents when working with code in this repository.

## Project Overview

TRH (Tokamak Rollup Hub) Backend - Go-based API server for Thanos rollup stack deployment and management.

**Tech Stack**: Go 1.24.11, Gin, GORM, PostgreSQL, JWT, trh-sdk

## Common Commands

```bash
# Local development
docker compose up -d              # Start PostgreSQL
go run main.go                    # Run server (PORT=8000)

# Build
go build -o trh-backend ./main.go

# Test
go test ./...
go test -v ./pkg/services/thanos/...  # Specific package

# Generate Swagger docs
swag init

# Lint
golangci-lint run
```

## Architecture

Clean Architecture pattern:

```
HTTP Request → Routes → Handlers → Services → Repositories → PostgreSQL
```

### Key Directories

| Path | Purpose |
|------|---------|
| `pkg/api/handlers/` | HTTP request handlers |
| `pkg/api/routes/route.go` | Route definitions and DI |
| `pkg/services/thanos/` | Stack deployment/termination business logic |
| `pkg/services/thanos/integrations/` | Integration features (Bridge, BlockExplorer, Monitoring, etc.) |
| `pkg/infrastructure/postgres/` | DB connection, repositories, schemas |
| `pkg/domain/entities/` | Entity and enum definitions |
| `pkg/taskmanager/` | Async task queue (Worker pool) |
| `internal/` | Internal utilities (logger, consts) |

### Core Components

1. **ThanosService** (`pkg/services/thanos/service.go`)
   - Stack CRUD, deployment, termination, updates
   - Uses `trh-sdk` library for infrastructure provisioning

2. **IntegrationManager** (`pkg/services/thanos/integrations/manager.go`)
   - Install/uninstall/update for each integration
   - Bridge, BlockExplorer, Monitoring, CrossTrade, UptimeService

3. **TaskManager** (`pkg/taskmanager/task_manager.go`)
   - Async job queue (5 workers, 20 buffer)
   - Context-based cancellation

4. **Server** (`pkg/api/servers/server.go`)
   - Graceful shutdown handling
   - Auto-stops in-progress deployments on shutdown

### Entity States

- **StackStatus**: Pending → Deploying → Deployed/Failed, Updating, Terminating → Terminated
- **DeploymentStatus**: Pending → InProgress → Completed/Failed/Cancelled

### API Structure

```
/api/v1/
├── /health                    # Public
├── /auth/                     # Authentication (login, profile)
├── /configuration/            # AWS credentials, RPC URL, API Key
└── /stacks/thanos/            # Stack management
    └── /:id/integrations/     # Integration management
```

## Environment Variables

See `.env.example`. Required:
- `POSTGRES_*`: DB connection info
- `JWT_SECRET`: JWT signing key
- `DEFAULT_ADMIN_*`: Initial admin account

## Code Conventions

- Repository interfaces: `pkg/services/thanos/interfaces.go`
- DTOs: `pkg/api/dtos/`
- Error responses: Use `dtos.ErrorResponse` in handlers
- Logging: `internal/logger` (Zap wrapper)
