# Preset Feature Implementation Spec

> Purpose: provide a repository-aligned specification that Codex can implement directly in `trh-backend`
> Scope: Phase 0 required items and Phase 2 optional follow-up
> Baseline date: 2026-03-10

## Goal

Add preset discovery, preset-based deployment input, and funding status lookup to the existing Thanos stack API without breaking current deploy or integration flows.

## Current Repository Constraints

- All Thanos APIs are mounted under `/api/v1/stacks/thanos` in [pkg/api/routes/route.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/routes/route.go#L1).
- Deploy input is `dtos.DeployThanosRequest` in [pkg/api/dtos/thanos.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/dtos/thanos.go#L47).
- Stack-level deploy config is persisted in `stacks.config` as the full deploy request JSON in [pkg/services/thanos/stack_lifecycle.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/stack_lifecycle.go#L18).
- Deployment step config is persisted in `deployments.config` as step-specific JSON in [pkg/services/thanos/helpers.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/helpers.go#L12).
- Multiple integrations rehydrate `dtos.DeployThanosRequest` from stored config, so newly added fields must remain backward-compatible.

## Recommended API Shape

### Phase 0

- `GET /api/v1/stacks/thanos/presets`
- `GET /api/v1/stacks/thanos/presets/:presetId`
- `POST /api/v1/stacks/thanos`
  - extend the existing deploy payload with `presetId` and `seedPhrase`
- `GET /api/v1/stacks/thanos/:id/funding-status`

### Phase 2

- `PUT /api/v1/stacks/thanos/:id/integrations/monitoring/config`
- `PUT /api/v1/stacks/thanos/:id/integrations/block-explorer/config`

## Route Placement Rules

- Register `/presets` routes before `/:id` routes in [pkg/api/routes/route.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/routes/route.go#L178) to avoid path collision.
- Preset routes should live in the authenticated read-only route group.
- Funding status should be stack-scoped, not deployment-scoped, because the authoritative account data comes from `stacks.config`.

## Data Model Decisions

### Extend `dtos.DeployThanosRequest`

Add these optional fields in [pkg/api/dtos/thanos.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/dtos/thanos.go#L47):

```go
PresetID   string `json:"presetId,omitempty"`
SeedPhrase string `json:"seedPhrase,omitempty"`
```

Rules:
- If `presetId` is empty, preserve current behavior.
- If `presetId` is present, merge preset defaults into the deployment config before validation.
- Explicit request fields win over preset defaults only for fields marked as overridable in the preset definition.
- `seedPhrase` is optional and must never be written to logs.

### Extend `entities.DeploymentEntity`

Add these fields in [pkg/domain/entities/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/domain/entities/deployment.go#L9):

```go
PresetID            string          `json:"preset_id,omitempty"`
SeedDerivedAccounts json.RawMessage `json:"seed_derived_accounts,omitempty"`
FundingStatus       json.RawMessage `json:"funding_status,omitempty"`
ModuleConfigs       json.RawMessage `json:"module_configs,omitempty"`
```

Mirror them in [pkg/infrastructure/postgres/schemas/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/infrastructure/postgres/schemas/deployment.go#L11) using `jsonb` for JSON payloads.

Update mapping in [pkg/infrastructure/postgres/repositories/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/infrastructure/postgres/repositories/deployment.go#L14).

## Preset Source of Truth

Create a dedicated package:

- `pkg/services/thanos/presets/types.go`
- `pkg/services/thanos/presets/service.go`

It should expose:

```go
type Definition struct {
    ID                string
    Name              string
    Description       string
    Modules           map[string]bool
    GenesisPredeploys []string
    EstimatedTime     map[string]string
    ChainDefaults     map[string]any
    HelmValues        map[string]any
    OverridableFields []string
}
```

Rules:
- The currently pinned `trh-sdk` version does not expose a preset package or `--preset` flag, so backend must own the initial preset definitions.
- Keep the values in backend code so the API remains available even if `trh-sdk` does not expose them at runtime.
- Add a validation method that rejects unknown preset IDs.

## Draft Preset Payloads

These are backend-owned draft definitions intended to unblock implementation. They should live in `pkg/services/thanos/presets/service.go` as static Go data.

```go
var DefaultPresetDefinitions = map[string]Definition{
    "general": {
        ID:          "general",
        Name:        "General Purpose",
        Description: "Baseline rollup preset for standard application workloads.",
        Modules: map[string]bool{
            "bridge":         true,
            "blockExplorer":  false,
            "monitoring":     false,
            "crossTrade":     false,
            "uptimeService":  false,
        },
        GenesisPredeploys: []string{
            "L2StandardBridge",
            "L2CrossDomainMessenger",
            "OptimismMintableERC20Factory",
        },
        EstimatedTime: map[string]string{
            "deploy":      "20-30m",
            "fundingWait": "5-15m",
        },
        ChainDefaults: map[string]any{
            "l2BlockTime":              2,
            "batchSubmissionFrequency": 1800,
            "outputRootFrequency":      1800,
            "challengePeriod":          86400,
            "registerCandidate":        false,
            "backupEnabled":            false,
        },
        HelmValues: map[string]any{
            "bridge.enabled":          true,
            "monitoring.enabled":      false,
            "blockscout.enabled":      false,
            "crossTrade.enabled":      false,
            "uptimeService.enabled":   false,
        },
        OverridableFields: []string{
            "l2BlockTime",
            "batchSubmissionFrequency",
            "outputRootFrequency",
            "challengePeriod",
            "backupEnabled",
        },
    },
    "defi": {
        ID:          "defi",
        Name:        "DeFi",
        Description: "Preset for exchange, liquidity, and settlement-heavy workloads.",
        Modules: map[string]bool{
            "bridge":         true,
            "blockExplorer":  true,
            "monitoring":     true,
            "crossTrade":     false,
            "uptimeService":  true,
        },
        GenesisPredeploys: []string{
            "L2StandardBridge",
            "L2CrossDomainMessenger",
            "OptimismMintableERC20Factory",
        },
        EstimatedTime: map[string]string{
            "deploy":      "30-40m",
            "fundingWait": "5-15m",
        },
        ChainDefaults: map[string]any{
            "l2BlockTime":              2,
            "batchSubmissionFrequency": 900,
            "outputRootFrequency":      900,
            "challengePeriod":          86400,
            "registerCandidate":        false,
            "backupEnabled":            true,
        },
        HelmValues: map[string]any{
            "bridge.enabled":          true,
            "monitoring.enabled":      true,
            "blockscout.enabled":      true,
            "crossTrade.enabled":      false,
            "uptimeService.enabled":   true,
        },
        OverridableFields: []string{
            "batchSubmissionFrequency",
            "outputRootFrequency",
            "challengePeriod",
        },
    },
    "gaming": {
        ID:          "gaming",
        Name:        "Gaming",
        Description: "Preset optimized for higher throughput and player-facing observability.",
        Modules: map[string]bool{
            "bridge":         true,
            "blockExplorer":  true,
            "monitoring":     true,
            "crossTrade":     true,
            "uptimeService":  true,
        },
        GenesisPredeploys: []string{
            "L2StandardBridge",
            "L2CrossDomainMessenger",
            "OptimismMintableERC20Factory",
        },
        EstimatedTime: map[string]string{
            "deploy":      "35-45m",
            "fundingWait": "5-15m",
        },
        ChainDefaults: map[string]any{
            "l2BlockTime":              2,
            "batchSubmissionFrequency": 300,
            "outputRootFrequency":      600,
            "challengePeriod":          86400,
            "registerCandidate":        false,
            "backupEnabled":            true,
        },
        HelmValues: map[string]any{
            "bridge.enabled":          true,
            "monitoring.enabled":      true,
            "blockscout.enabled":      true,
            "crossTrade.enabled":      true,
            "uptimeService.enabled":   true,
        },
        OverridableFields: []string{
            "l2BlockTime",
            "batchSubmissionFrequency",
            "outputRootFrequency",
        },
    },
    "full": {
        ID:          "full",
        Name:        "Full Suite",
        Description: "All recommended modules enabled for demos, staging, or high-touch managed environments.",
        Modules: map[string]bool{
            "bridge":         true,
            "blockExplorer":  true,
            "monitoring":     true,
            "crossTrade":     true,
            "uptimeService":  true,
        },
        GenesisPredeploys: []string{
            "L2StandardBridge",
            "L2CrossDomainMessenger",
            "OptimismMintableERC20Factory",
        },
        EstimatedTime: map[string]string{
            "deploy":      "40-50m",
            "fundingWait": "5-15m",
        },
        ChainDefaults: map[string]any{
            "l2BlockTime":              2,
            "batchSubmissionFrequency": 600,
            "outputRootFrequency":      600,
            "challengePeriod":          86400,
            "registerCandidate":        true,
            "backupEnabled":            true,
        },
        HelmValues: map[string]any{
            "bridge.enabled":          true,
            "monitoring.enabled":      true,
            "blockscout.enabled":      true,
            "crossTrade.enabled":      true,
            "uptimeService.enabled":   true,
        },
        OverridableFields: []string{
            "l2BlockTime",
            "batchSubmissionFrequency",
            "outputRootFrequency",
            "challengePeriod",
            "registerCandidate",
        },
    },
}
```

### Payload Interpretation Rules

- `Modules` controls what the preset summary API returns.
- `ChainDefaults` is the only part merged into `dtos.DeployThanosRequest`.
- `HelmValues` is metadata for Phase 2 and should not change Phase 0 deployment behavior by itself.
- Environment-specific inputs such as `L1RpcUrl`, `L1BeaconUrl`, AWS credentials, and `ChainName` are never preset-owned.
- If a module is enabled in a preset, actual installation still follows the existing backend workflow. Phase 0 does not auto-install extra integrations unless explicitly implemented.

## Handler and Service Placement

### New handler file

- `pkg/api/handlers/thanos/presets.go`

Responsibilities:
- `ListPresets`
- `GetPresetByID`
- `GetFundingStatus`

### Existing handler updates

- [pkg/api/handlers/thanos/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/handlers/thanos/deployment.go#L15)

Responsibilities:
- bind new fields
- sanitize `seedPhrase`
- call preset-aware deploy service

### Service updates

- [pkg/services/thanos/stack_lifecycle.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/stack_lifecycle.go#L18)
- [pkg/services/thanos/helpers.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/helpers.go#L12)
- [pkg/services/thanos/service.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/service.go#L1)

Responsibilities:
- resolve preset definitions
- derive effective deploy config
- optionally derive EOA addresses from `seedPhrase`
- persist derived data in deployment rows
- expose a funding status service method

## Funding Status Contract

### Response shape

```go
type FundingStatusResponse struct {
    StackID      string           `json:"stack_id"`
    Network      string           `json:"network"`
    Accounts     []AccountFunding `json:"accounts"`
    AllFulfilled bool             `json:"allFulfilled"`
    CheckedAt    string           `json:"checkedAt"`
}

type AccountFunding struct {
    Role      string `json:"role"`
    Address   string `json:"address"`
    Required  string `json:"required"`
    Current   string `json:"current"`
    Fulfilled bool   `json:"fulfilled"`
}
```

### Source rules

- Addresses come from the effective stack config, not from the latest deployment step config.
- `Required` values must be defined in backend constants, versioned in code, and documented per network.
- Use the stack network plus `L1RpcUrl` from persisted stack config to query balances.
- Cache is optional for Phase 0. A direct RPC read per request is acceptable.
- Save the latest computed snapshot into `deployments.funding_status` only as a convenience cache, not as the source of truth.

### Initial required balances

Until product supplies another table, hardcode a temporary backend map:

```go
var RequiredFundingByNetwork = map[entities.DeploymentNetwork]map[string]string{
    entities.DeploymentNetworkTestnet: {
        "admin":     "...",
        "sequencer": "...",
        "batcher":   "...",
        "proposer":  "...",
    },
    entities.DeploymentNetworkMainnet: {
        "admin":     "...",
        "sequencer": "...",
        "batcher":   "...",
        "proposer":  "...",
    },
}
```

Before implementation, replace `...` with product-approved wei values.

## Seed Phrase Handling Rules

- If `seedPhrase` is set, derive accounts before deployment creation.
- Derived accounts must populate the existing role fields used by deployment and integration code.
- Persist only derived addresses and metadata required for later display.
- Never persist the raw seed phrase to the database.
- Never write the raw seed phrase to logs or error messages.
- If seed derivation fails, return `400 Bad Request`.

## Migration Scope

Update [pkg/infrastructure/postgres/connection/connection.go](/Users/theo/workspace_tokamak/trh-backend/pkg/infrastructure/postgres/connection/connection.go#L50) via schema changes handled by `AutoMigrate`.

Required schema changes:
- `deployments.preset_id`
- `deployments.seed_derived_accounts`
- `deployments.funding_status`
- `deployments.module_configs`

Backward compatibility rules:
- existing rows must remain readable
- empty JSON fields must deserialize safely
- integrations that unmarshal old `DeployThanosRequest` JSON must continue to work

## Implementation Tasks

### Task 1: Preset domain package

Files:
- Create `pkg/services/thanos/presets/types.go`
- Create `pkg/services/thanos/presets/service.go`

Definition of done:
- can list all presets
- can fetch one preset by ID
- invalid preset ID returns explicit error

### Task 2: Preset API

Files:
- Create `pkg/api/handlers/thanos/presets.go`
- Modify [pkg/api/routes/route.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/routes/route.go#L178)

Definition of done:
- authenticated user can call preset endpoints
- `/presets` does not conflict with `/:id`

### Task 3: Deploy request extension

Files:
- Modify [pkg/api/dtos/thanos.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/dtos/thanos.go#L47)
- Modify [pkg/api/handlers/thanos/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/handlers/thanos/deployment.go#L31)
- Modify [pkg/services/thanos/stack_lifecycle.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/stack_lifecycle.go#L18)
- Modify [pkg/services/thanos/helpers.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/helpers.go#L12)

Definition of done:
- old payload still deploys
- payload with `presetId` deploys
- payload with `seedPhrase` derives addresses safely
- persisted stack config remains compatible with existing integration code

### Task 4: Deployment persistence update

Files:
- Modify [pkg/domain/entities/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/domain/entities/deployment.go#L9)
- Modify [pkg/infrastructure/postgres/schemas/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/infrastructure/postgres/schemas/deployment.go#L11)
- Modify [pkg/infrastructure/postgres/repositories/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/infrastructure/postgres/repositories/deployment.go#L14)

Definition of done:
- new fields round-trip through repository methods
- old records still load

### Task 5: Funding status API

Files:
- Create `pkg/services/thanos/funding.go`
- Create or extend `pkg/api/handlers/thanos/presets.go`
- Modify [pkg/api/routes/route.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/routes/route.go#L178)

Definition of done:
- returns role-based balance data for a stack
- returns `404` when stack not found
- returns `400` when stack config is missing required addresses

### Task 6: Swagger and docs

Files:
- Update handler annotations in the new and changed handler files
- Regenerate `docs/swagger.json`, `docs/swagger.yaml`, and `docs/docs.go`

Definition of done:
- new endpoints appear in Swagger

## Verification Plan

### Unit tests to add

- `pkg/services/thanos/presets/service_test.go`
  - list presets
  - get preset by ID
  - invalid preset ID
- `pkg/api/handlers/thanos/presets_test.go`
  - list presets success
  - get preset detail success
  - funding status success with mocked service dependencies
- `pkg/infrastructure/postgres/repositories/deployment_test.go`
  - deployment round-trip with new JSON fields
- `pkg/services/thanos/funding_test.go`
  - all accounts funded
  - one account underfunded
  - invalid RPC or malformed address handling

### Commands

```bash
go test ./...
go test -v ./pkg/services/thanos/...
go test -v ./pkg/api/handlers/thanos/...
go test -v ./pkg/infrastructure/postgres/repositories/...
go build -o trh-backend ./main.go
swag init
```

If `golangci-lint` is available in the environment:

```bash
golangci-lint run
```

## Open Product Decisions

- exact funding thresholds per network
- whether explicit request fields may override preset defaults
- whether Phase 2 module config should reuse existing integration update endpoints instead of new config endpoints

## Codex Readiness

This refined document is sufficient for Codex to start implementation with bounded assumptions, except for two product-owned values that still need confirmation:

- the concrete per-network required balances in wei
- whether the draft preset payloads above should be accepted as-is or adjusted by product

Everything else is now mapped to actual repository files and route structure.
