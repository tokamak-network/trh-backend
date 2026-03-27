# 데이터 모델 — LocalTestnet 배포

**버전**: 1.0
**작성일**: 2026-03-10

---

## 1. 변경되는 엔티티

### DeploymentNetwork (enum 추가)

```go
// pkg/domain/entities/enums.go
type DeploymentNetwork string

const (
    Mainnet      DeploymentNetwork = "mainnet"
    Testnet      DeploymentNetwork = "testnet"
    LocalDevnet  DeploymentNetwork = "local_devnet"  // 기존
    LocalTestnet DeploymentNetwork = "local_testnet" // 신규
)
```

### DeploymentStep 상수 (추가)

```go
// pkg/constants/deployment.go
const (
    DeployL1Contracts  = "deploy-l1-contracts"  // 기존
    DeployAWSInfra     = "deploy-aws-infra"      // 기존
    DeployLocalInfra   = "deploy-local-infra"    // 신규 — kind + Helm
    DestroyChain       = "destroy-chain"         // 기존
    // ... 기타 기존 상수 유지
)
```

### StackEntity (필드 추가)

```go
// pkg/domain/entities/stack.go
type StackEntity struct {
    // 기존 필드 유지
    ID              uuid.UUID
    Name            string
    Network         DeploymentNetwork
    Config          json.RawMessage
    DeploymentPath  string
    Metadata        *StackMetadata
    Status          StackStatus
    CreatedAt       time.Time
    UpdatedAt       time.Time

    // 신규 필드
    KubeconfigPath  string  // LocalTestnet 시 kind kubeconfig 파일 경로
    SeedPhrase      string  // 암호화 저장, 선택적 (키 자동 생성 시 사용)
}
```

---

## 2. 신규 엔티티

### FundingStatusEntity

```go
// pkg/domain/entities/funding.go (신규 파일)
type FundingStatusEntity struct {
    ID         uuid.UUID
    StackID    uuid.UUID
    Deployer   EOAFunding
    Batcher    EOAFunding
    Proposer   EOAFunding
    Challenger EOAFunding
    UpdatedAt  time.Time
}

type EOAFunding struct {
    Address   string   // EOA 주소 (0x...)
    Required  string   // 필요 금액 (wei 문자열)
    Current   string   // 현재 잔액 (wei 문자열)
    Fulfilled bool     // 충족 여부
}
```

---

## 3. API DTO 변경

### DeployThanosRequest (수정)

```go
// pkg/api/dtos/thanos_dto.go
type DeployThanosRequest struct {
    // 기존 필드
    ChainName string            `json:"chainName"`
    Network   DeploymentNetwork `json:"network"`
    L1RPCURL  string            `json:"l1RpcUrl"`
    // ...

    // LocalTestnet 시 필수, 그 외 optional
    AWSAccessKey    *string `json:"awsAccessKey,omitempty"`
    AWSSecretKey    *string `json:"awsSecretAccessKey,omitempty"`
    AWSRegion       *string `json:"awsRegion,omitempty"`

    // LocalTestnet 전용 필드 (신규)
    KubeconfigPath  *string `json:"kubeconfigPath,omitempty"`
    SeedPhrase      *string `json:"seedPhrase,omitempty"`
}
```

### FundingStatusResponse (신규)

```go
// pkg/api/dtos/funding_dto.go (신규 파일)
type FundingStatusResponse struct {
    StackID    string         `json:"stackId"`
    Deployer   EOAFundingDTO  `json:"deployer"`
    Batcher    EOAFundingDTO  `json:"batcher"`
    Proposer   EOAFundingDTO  `json:"proposer"`
    Challenger EOAFundingDTO  `json:"challenger"`
    AllFulfilled bool         `json:"allFulfilled"`
    UpdatedAt  time.Time      `json:"updatedAt"`
}

type EOAFundingDTO struct {
    Address    string `json:"address"`
    RequiredWei string `json:"requiredWei"`
    CurrentWei  string `json:"currentWei"`
    Fulfilled   bool   `json:"fulfilled"`
}
```

---

## 4. 엔티티 관계도

```
StackEntity (1)
  ├── DeploymentEntity[] (N)   ← deploy-l1-contracts, deploy-local-infra
  ├── IntegrationEntity[] (N)  ← Bridge, Explorer 등 (기존)
  └── FundingStatusEntity (1)  ← 신규, LocalTestnet 배포 시 생성
```

---

## 5. DB 마이그레이션

```sql
-- stacks 테이블 컬럼 추가
ALTER TABLE stacks ADD COLUMN kubeconfig_path TEXT;
ALTER TABLE stacks ADD COLUMN seed_phrase TEXT; -- 암호화 저장

-- funding_status 테이블 신규 생성
CREATE TABLE funding_statuses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stack_id UUID NOT NULL REFERENCES stacks(id),
    deployer JSONB NOT NULL,
    batcher JSONB NOT NULL,
    proposer JSONB NOT NULL,
    challenger JSONB NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```
