# Preset 기능 구현 스펙

> 목적: Codex가 `trh-backend` 저장소에서 바로 구현에 착수할 수 있도록 저장소 구조에 맞춘 실행 스펙 제공
> 범위: Phase 0 필수 기능과 Phase 2 후속 기능
> 기준일: 2026-03-10

## 목표

기존 Thanos stack API에 preset 조회, preset 기반 배포 입력, funding status 조회 기능을 추가하되 현재 deploy 및 integration 흐름을 깨지 않도록 한다.

## 현재 저장소 제약

- 모든 Thanos API는 [pkg/api/routes/route.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/routes/route.go#L1) 기준 `/api/v1/stacks/thanos` 아래에 마운트된다.
- 배포 입력 DTO는 [pkg/api/dtos/thanos.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/dtos/thanos.go#L47)의 `dtos.DeployThanosRequest`이다.
- stack 레벨 배포 설정은 [pkg/services/thanos/stack_lifecycle.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/stack_lifecycle.go#L18)에서 전체 deploy request JSON 형태로 `stacks.config`에 저장된다.
- deployment step 설정은 [pkg/services/thanos/helpers.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/helpers.go#L12)에서 step별 JSON 형태로 `deployments.config`에 저장된다.
- 여러 integration 코드가 저장된 config를 다시 `dtos.DeployThanosRequest`로 역직렬화해서 사용하므로, 새 필드는 반드시 하위호환성을 유지해야 한다.

## 권장 API 형태

### Phase 0

- `GET /api/v1/stacks/thanos/presets`
- `GET /api/v1/stacks/thanos/presets/:presetId`
- `POST /api/v1/stacks/thanos`
  - 기존 deploy payload에 `presetId`, `seedPhrase` 추가
- `GET /api/v1/stacks/thanos/:id/funding-status`

### Phase 2

- `PUT /api/v1/stacks/thanos/:id/integrations/monitoring/config`
- `PUT /api/v1/stacks/thanos/:id/integrations/block-explorer/config`

## 라우트 배치 규칙

- [pkg/api/routes/route.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/routes/route.go#L178)에서 `/presets` 라우트는 `/:id` 라우트보다 먼저 등록해야 충돌이 없다.
- preset 조회 API는 인증된 사용자용 read-only route group에 둔다.
- funding status는 deployment 기준이 아니라 stack 기준으로 둔다. 실제 계정 정보의 원본은 `stacks.config`이기 때문이다.

## 데이터 모델 결정

### `dtos.DeployThanosRequest` 확장

[pkg/api/dtos/thanos.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/dtos/thanos.go#L47)에 아래 optional 필드를 추가한다.

```go
PresetID   string `json:"presetId,omitempty"`
SeedPhrase string `json:"seedPhrase,omitempty"`
```

규칙:
- `presetId`가 비어 있으면 현재 동작을 그대로 유지한다.
- `presetId`가 있으면 validation 전에 preset default를 배포 설정에 병합한다.
- preset 정의에서 override 가능으로 표시한 필드만 요청값이 preset default를 덮어쓸 수 있다.
- `seedPhrase`는 optional이며 어떤 경우에도 로그에 남기지 않는다.

### `entities.DeploymentEntity` 확장

[pkg/domain/entities/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/domain/entities/deployment.go#L9)에 아래 필드를 추가한다.

```go
PresetID            string          `json:"preset_id,omitempty"`
SeedDerivedAccounts json.RawMessage `json:"seed_derived_accounts,omitempty"`
FundingStatus       json.RawMessage `json:"funding_status,omitempty"`
ModuleConfigs       json.RawMessage `json:"module_configs,omitempty"`
```

[pkg/infrastructure/postgres/schemas/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/infrastructure/postgres/schemas/deployment.go#L11)에도 같은 필드를 추가하고 JSON 필드는 `jsonb`로 저장한다.

[pkg/infrastructure/postgres/repositories/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/infrastructure/postgres/repositories/deployment.go#L14)의 매핑도 함께 수정한다.

## Preset 원본 데이터

전용 패키지를 만든다.

- `pkg/services/thanos/presets/types.go`
- `pkg/services/thanos/presets/service.go`

노출 타입 예시:

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

규칙:
- 현재 고정된 `trh-sdk` 버전에는 preset 패키지나 `--preset` 플래그가 없으므로, 초기 preset 정의는 backend가 소유한다.
- 런타임에 `trh-sdk`가 preset 데이터를 직접 노출하지 않더라도 API가 동작하도록 backend 코드 내부에 값을 보관한다.
- 알 수 없는 preset ID는 validation 단계에서 명시적으로 거부한다.

## Preset 초안 Payload

아래 값은 구현을 막지 않기 위한 backend-owned 초안이다. `pkg/services/thanos/presets/service.go`에 정적 Go 데이터로 둔다.

```go
var DefaultPresetDefinitions = map[string]Definition{
    "general": {
        ID:          "general",
        Name:        "General Purpose",
        Description: "일반적인 애플리케이션 워크로드를 위한 기본 롤업 preset.",
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
        Description: "거래소, 유동성, 정산 중심 워크로드를 위한 preset.",
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
        Description: "더 높은 처리량과 플레이어 대상 가시성을 위한 preset.",
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
        Description: "데모, 스테이징, 관리형 운영 환경을 위한 전체 권장 모듈 preset.",
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

### Payload 해석 규칙

- `Modules`는 preset summary API 응답 용도다.
- `ChainDefaults`만 `dtos.DeployThanosRequest`에 실제 병합된다.
- `HelmValues`는 Phase 2용 메타데이터이며, 그것만으로 Phase 0 배포 동작을 바꾸지 않는다.
- `L1RpcUrl`, `L1BeaconUrl`, AWS credential, `ChainName` 같은 환경 의존 입력은 preset이 소유하지 않는다.
- preset에서 모듈이 활성화되어도 실제 설치는 기존 backend workflow를 따른다. Phase 0에서는 명시 구현 없이는 추가 integration을 자동 설치하지 않는다.

## 핸들러 및 서비스 배치

### 신규 핸들러 파일

- `pkg/api/handlers/thanos/presets.go`

역할:
- `ListPresets`
- `GetPresetByID`
- `GetFundingStatus`

### 기존 핸들러 수정

- [pkg/api/handlers/thanos/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/handlers/thanos/deployment.go#L15)

역할:
- 새 필드 bind
- `seedPhrase` sanitize
- preset-aware deploy service 호출

### 서비스 수정

- [pkg/services/thanos/stack_lifecycle.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/stack_lifecycle.go#L18)
- [pkg/services/thanos/helpers.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/helpers.go#L12)
- [pkg/services/thanos/service.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/service.go#L1)

역할:
- preset 정의 조회
- 최종 deploy config 계산
- 필요 시 `seedPhrase`로 EOA 주소 파생
- deployment row에 파생 데이터 저장
- funding status 조회 서비스 메서드 제공

## Funding Status 명세

### 응답 형태

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

### 데이터 원칙

- 주소 정보는 최신 deployment step config가 아니라 최종 stack config에서 가져온다.
- `Required` 값은 backend 상수로 관리하고, 네트워크별로 코드에 버전 관리한다.
- 잔액 조회는 저장된 stack config의 `L1RpcUrl`과 stack network를 사용한다.
- Phase 0에서는 캐시 없이 요청당 직접 RPC 조회로 시작해도 된다.
- 계산한 최신 snapshot을 `deployments.funding_status`에 저장할 수는 있지만, 이것은 편의용 캐시일 뿐 원본 데이터로 취급하지 않는다.

### 초기 required balance

별도 제품 정책표가 나오기 전까지는 임시로 backend map을 둔다.

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

구현 전 반드시 `...`를 제품 승인된 wei 값으로 교체한다.

## Seed Phrase 처리 규칙

- `seedPhrase`가 있으면 deployment 생성 전에 계정을 파생한다.
- 파생 계정은 기존 deployment 및 integration 코드가 쓰는 role 필드를 채워야 한다.
- DB에는 이후 표시용으로 필요한 파생 주소와 메타데이터만 저장한다.
- raw seed phrase는 DB에 저장하지 않는다.
- raw seed phrase는 로그와 에러 메시지에 절대 남기지 않는다.
- seed 파생 실패 시 `400 Bad Request`를 반환한다.

## 마이그레이션 범위

[pkg/infrastructure/postgres/connection/connection.go](/Users/theo/workspace_tokamak/trh-backend/pkg/infrastructure/postgres/connection/connection.go#L50)의 `AutoMigrate` 흐름을 통해 스키마 변경을 반영한다.

필수 컬럼:
- `deployments.preset_id`
- `deployments.seed_derived_accounts`
- `deployments.funding_status`
- `deployments.module_configs`

하위호환 규칙:
- 기존 row는 그대로 읽혀야 한다
- 비어 있는 JSON 필드는 안전하게 역직렬화돼야 한다
- 오래된 `DeployThanosRequest` JSON을 역직렬화하는 integration 코드는 계속 동작해야 한다

## 구현 작업

### Task 1: Preset 도메인 패키지

파일:
- 생성 `pkg/services/thanos/presets/types.go`
- 생성 `pkg/services/thanos/presets/service.go`

완료 기준:
- 모든 preset 목록 조회 가능
- ID 기준 단건 조회 가능
- 잘못된 preset ID는 명시적 에러 반환

### Task 2: Preset API

파일:
- 생성 `pkg/api/handlers/thanos/presets.go`
- 수정 [pkg/api/routes/route.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/routes/route.go#L178)

완료 기준:
- 인증 사용자로 preset endpoint 호출 가능
- `/presets`가 `/:id`와 충돌하지 않음

### Task 3: Deploy request 확장

파일:
- 수정 [pkg/api/dtos/thanos.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/dtos/thanos.go#L47)
- 수정 [pkg/api/handlers/thanos/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/handlers/thanos/deployment.go#L31)
- 수정 [pkg/services/thanos/stack_lifecycle.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/stack_lifecycle.go#L18)
- 수정 [pkg/services/thanos/helpers.go](/Users/theo/workspace_tokamak/trh-backend/pkg/services/thanos/helpers.go#L12)

완료 기준:
- 기존 payload로도 deploy 가능
- `presetId` 포함 payload로 deploy 가능
- `seedPhrase` 포함 payload는 안전하게 주소를 파생함
- 저장된 stack config가 기존 integration 코드와 호환됨

### Task 4: Deployment 영속 필드 확장

파일:
- 수정 [pkg/domain/entities/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/domain/entities/deployment.go#L9)
- 수정 [pkg/infrastructure/postgres/schemas/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/infrastructure/postgres/schemas/deployment.go#L11)
- 수정 [pkg/infrastructure/postgres/repositories/deployment.go](/Users/theo/workspace_tokamak/trh-backend/pkg/infrastructure/postgres/repositories/deployment.go#L14)

완료 기준:
- 새 필드가 repository round-trip을 통과함
- 기존 record도 문제없이 로드됨

### Task 5: Funding Status API

파일:
- 생성 `pkg/services/thanos/funding.go`
- 생성 또는 확장 `pkg/api/handlers/thanos/presets.go`
- 수정 [pkg/api/routes/route.go](/Users/theo/workspace_tokamak/trh-backend/pkg/api/routes/route.go#L178)

완료 기준:
- stack 기준 role별 balance 반환
- stack이 없으면 `404`
- stack config에 필수 주소가 없으면 `400`

### Task 6: Swagger 및 문서

파일:
- 신규 및 수정 핸들러 annotation 업데이트
- `docs/swagger.json`, `docs/swagger.yaml`, `docs/docs.go` 재생성

완료 기준:
- 새 endpoint가 Swagger에 노출됨

## 검증 계획

### 추가할 unit test

- `pkg/services/thanos/presets/service_test.go`
  - preset 목록 조회
  - preset 단건 조회
  - 잘못된 preset ID 처리
- `pkg/api/handlers/thanos/presets_test.go`
  - preset 목록 성공
  - preset 상세 성공
  - mocked service 의존성 기반 funding status 성공
- `pkg/infrastructure/postgres/repositories/deployment_test.go`
  - 새 JSON 필드를 포함한 deployment round-trip
- `pkg/services/thanos/funding_test.go`
  - 모든 계정 funding 충족
  - 한 계정 부족
  - RPC 오류 또는 잘못된 주소 처리

### 실행 명령

```bash
go test ./...
go test -v ./pkg/services/thanos/...
go test -v ./pkg/api/handlers/thanos/...
go test -v ./pkg/infrastructure/postgres/repositories/...
go build -o trh-backend ./main.go
swag init
```

환경에 `golangci-lint`가 있다면 추가 실행:

```bash
golangci-lint run
```

## 남은 제품 결정 사항

- 네트워크별 정확한 funding threshold
- 요청 필드가 preset default를 override 가능한지 여부
- Phase 2 module config가 신규 config endpoint를 써야 하는지, 아니면 기존 integration update 흐름을 재사용해야 하는지 여부

## Codex 실행 가능성 평가

이 정제 문서는 두 가지 제품 결정값만 확정되면 Codex가 bounded assumption으로 바로 구현에 착수할 수 있는 수준이다.

- 네트워크별 required balance의 정확한 wei 값
- 위 초안 preset payload를 제품안으로 수용할지, 조정할지 여부

그 외 항목은 실제 저장소 파일과 라우트 구조에 매핑되어 있다.
