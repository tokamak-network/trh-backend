# Funding Status API Design

**Date**: 2026-03-16
**Phase**: 3 (remaining)
**Scope**: `GET /api/v1/stacks/thanos/:id/funding-status`

## Overview

Real-time L1 balance query for 4 operator EOAs derived from a stack's seed phrase. No DB persistence — each request queries L1 RPC directly.

## Architecture

```
GET /stacks/thanos/:id/funding-status
  → Handler: extract stack ID
  → Service: load stack → derive keys (BIP44) → query L1 balances (parallel) → compare vs required
  → Response: per-EOA funding status + allFulfilled flag
```

## Response Shape

```json
{
  "status": 200,
  "message": "success",
  "data": {
    "deployer":   { "address": "0x...", "requiredWei": "500000000000000000", "currentWei": "123...", "balanceEth": "0.12...", "fulfilled": false },
    "batcher":    { "address": "0x...", "requiredWei": "200000000000000000", "currentWei": "...", "balanceEth": "...", "fulfilled": false },
    "proposer":   { "address": "0x...", "requiredWei": "100000000000000000", "currentWei": "...", "balanceEth": "...", "fulfilled": true },
    "challenger": { "address": "0x...", "requiredWei": "100000000000000000", "currentWei": "...", "balanceEth": "...", "fulfilled": false },
    "allFulfilled": false
  }
}
```

## Files Changed

| File | Type | Description |
|------|------|-------------|
| `pkg/api/dtos/funding.go` | New | FundingStatusResponse, EOAFunding DTOs |
| `pkg/services/thanos/funding.go` | New | GetFundingStatus() — key derivation + balance query + fulfilled calc |
| `pkg/api/handlers/thanos/keyderivation.go` | Edit | Add GetFundingStatus handler method |
| `pkg/api/routes/route.go` | Edit | Register GET /:id/funding-status |

## Key Decisions

- **No DB** — real-time RPC query only
- **Required amounts hardcoded** — deployer 0.5 ETH, batcher 0.2 ETH, proposer 0.1 ETH, challenger 0.1 ETH
- **Partial failure tolerant** — individual EOA query failure returns error field, doesn't fail whole request
- **Reuses** existing `keyderivation.DeriveOperatorKeys()`, `fetchBalancesParallel()`, `weiToEth()`
- **Auth**: authenticated route (not admin-only)
