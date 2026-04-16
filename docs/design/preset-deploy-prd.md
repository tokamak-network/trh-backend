# PRD: Preset-Based L2 Deployment — Backend

```
Status: In Progress
Date: 2026-04-15
Owner: trh-platform team
Relates-to:
  - docs/design/preset-module-install-aws.md (ADR ④ — installPresetModules 오케스트레이션)
  - docs/design/credential-storage.md (ADR ③ — stateless credential 주입)
  - trh-sdk/pkg/constants/chain.go (PresetModules — 단일 권위 소스)
  - trh-platform-ui/docs/preset-deploy-prd.md (PRD 5 — wizard 5 필드)
```

---

## Background

Preset scaffolding은 `pkg/services/thanos/presets/service.go`(`DefaultPresetDefinitions`)와
`preset_deploy.go:18 CreateThanosStackFromPreset`에 이미 존재하고
`POST /stacks/thanos/preset-deploy` 엔드포인트도 동작한다. 다만 세 가지 문제가 있다:

1. **이중 소스 문제**: backend의 `Modules` 맵이 trh-sdk `pkg/constants/chain.go:PresetModules`와
   별도로 관리되어 drift 위험이 있다.

2. **DRB install 핸들러 없음**: `integrations.go:UninstallDRB`는 있지만 `InstallDRB` 핸들러와
   라우트가 없어서 preset 경로 밖에서 DRB를 배포할 수 없다.

3. **페이로드 입력 필드 과다**: `DeployWithPresetRequest`가 많은 필드를 required로 요구한다.
   UI에서 5 필드(AWS Access Key, AWS Secret Key, Preset, Chain Name, Network)만 받으려면
   backend validation도 완화해야 한다.

---

## Scope

### In scope
- `POST /stacks/thanos/preset-deploy` 페이로드를 5 필드만 필수로 수용
- `POST /:id/integrations/drb` install 핸들러 + 라우트 추가
- Preset → Modules 단일 권위를 SDK `constants.PresetModules`로 통합

### Out of scope
- `POST /integrations/staking-v2` — TRH 생태계 integration 아님
- Backup REST 핸들러 신규 — AWS Backup 서비스 경로 유지
- 기존 `POST /:id/integrations/{bridge,block-explorer,monitoring,cross-trade}` — 유지

---

## Deliverables

### 1. `pkg/services/thanos/presets/service.go` 수정

`DefaultPresetDefinitions.Modules` 필드를 trh-sdk `constants.PresetModules`에서
재파생하도록 변경. backend가 별도로 모듈 목록을 관리하지 않는다.

```go
// presets/service.go
import sdkConstants "github.com/tokamak-network/trh-sdk/pkg/constants"

func buildPresetDefinition(presetID string) PresetDefinition {
    return PresetDefinition{
        ID:      presetID,
        Modules: sdkConstants.PresetModules[presetID], // 단일 권위
    }
}
```

### 2. `pkg/api/handlers/thanos/integrations.go` — InstallDRB 핸들러 추가

`UninstallDRB`(L800) 패턴을 미러링하여 `InstallDRB` 핸들러 신규:

```go
// InstallBridge 패턴 미러
func (h *ThanosIntegrationHandler) InstallDRB(c *gin.Context) {
    stackID := c.Param("id")
    // ADR ③ stateless credential 주입 방식
    creds, err := h.extractAWSCredentials(c)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    task, err := h.service.InstallDRB(c.Request.Context(), stackID, creds)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusAccepted, task)
}
```

`h.service.InstallDRB`는 trh-sdk의 `ThanosStack.InstallDRB(ctx)`를 호출한다.

### 3. `pkg/api/routes/route.go` — DRB 라우트 추가

```go
// 기존 integrations 그룹에 추가
integrations.POST("/drb", handler.InstallDRB)
integrations.DELETE("/drb", handler.UninstallDRB)  // 이미 있음
```

### 4. `pkg/api/dtos/thanos.go` — DeployWithPresetRequest validation 조정

```go
type DeployWithPresetRequest struct {
    // 필수 5 필드
    PresetID        string `json:"presetId"        validate:"required"`
    ChainName       string `json:"chainName"       validate:"required"`
    Network         string `json:"network"         validate:"required,oneof=testnet mainnet"`
    AWSAccessKey    string `json:"awsAccessKey"    validate:"required"`
    AWSSecretKey    string `json:"awsSecretKey"    validate:"required"`

    // optional — 누락 시 preset default 또는 keystore 파생으로 채움
    AWSRegion       string `json:"awsRegion"       validate:"omitempty"`
    AWSSessionToken string `json:"awsSessionToken" validate:"omitempty"`
    // 아래 필드들: 기존 필드 유지하되 required 태그 제거
    L1RpcUrl        string `json:"l1RpcUrl"        validate:"omitempty,url"`
    L1BeaconUrl     string `json:"l1BeaconUrl"     validate:"omitempty,url"`
    FeeToken        string `json:"feeToken"        validate:"omitempty"`
    AdminPrivateKey string `json:"adminPrivateKey" validate:"omitempty"`
}
```

### 5. `pkg/services/thanos/preset_deploy.go` 수정

`CreateThanosStackFromPreset` — optional 필드 누락 시 preset default로 채우는 로직:

- `AWSRegion` 누락 → AWS SDK default chain (env `AWS_DEFAULT_REGION` → profile → us-east-1)
- `FeeToken` 누락 → `"TON"` (기본값)
- `AdminPrivateKey` 누락 → keystore에서 BIP44 파생 (trh-platform이 주입하지 않는 경우 fallback)

---

## API 계약

### `POST /stacks/thanos/preset-deploy`

**Request (5 필드 최소):**
```json
{
  "presetId": "full",
  "chainName": "my-chain",
  "network": "testnet",
  "awsAccessKey": "AKIA...",
  "awsSecretKey": "..."
}
```

**Response (202 Accepted):**
```json
{
  "taskId": "task-abc123",
  "stackId": "stack-xyz789",
  "status": "pending"
}
```

### `POST /api/v1/stacks/{id}/integrations/drb`

**Request:**
```json
{
  "awsAccessKey": "AKIA...",
  "awsSecretKey": "...",
  "awsSessionToken": "..."   // optional, STS temporary credentials
}
```

**Response (202 Accepted):**
```json
{
  "taskId": "task-drb-001",
  "status": "installing"
}
```

---

## Verification

- [ ] `go test ./pkg/api/... ./pkg/services/thanos/...` 통과
- [ ] `POST /stacks/thanos/preset-deploy` with 5 필드 → 200, task 생성 확인
- [ ] `POST /:id/integrations/drb` 라우트 등록 확인 (`curl /:id/integrations/drb` 404 아님)
- [ ] `PresetModules` 정보가 SDK constants와 일치하는지 통합 테스트
- [ ] `AWSRegion` 누락 시 default region fallback 동작 확인

---

## Implementation Checklist

- [ ] `pkg/services/thanos/presets/service.go` — `Modules` 필드를 SDK constants에서 재파생
- [ ] `pkg/api/handlers/thanos/integrations.go` — `InstallDRB` 핸들러 신규 (L800 `UninstallDRB` 패턴 미러)
- [ ] `pkg/api/routes/route.go` — `POST /:id/integrations/drb` 라우트 추가
- [ ] `pkg/api/dtos/thanos.go` — `DeployWithPresetRequest` 5 필드만 required, 나머지 omitempty
- [ ] `pkg/services/thanos/preset_deploy.go` — optional 필드 누락 시 default 채움 로직
- [ ] ADR ④ `preset-module-install-aws.md` implementation checklist 연동 진행
- [ ] Postman collection 업데이트 (preset-deploy 5 필드 예제)
- [ ] 구현 PR merge 후 ADR ④ `Status: Accepted` 업데이트
