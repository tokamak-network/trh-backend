# 프로젝트 스펙 — LocalTestnet 배포

**버전**: 1.0
**작성일**: 2026-03-10

---

## 1. AI 행동 규칙

### 절대 규칙

1. **기존 Mainnet/Testnet 로직 변경 금지**
   - `network != "local_testnet"` 인 경우 기존 코드 경로를 절대 수정하지 않는다
   - 분기 로직은 항상 `if network == LocalTestnet { ... }` 형태로 명시적으로 분리

2. **LocalTestnet은 Testnet 전용**
   - `LocalTestnet` 배포 요청에 `network: "mainnet"` 이 섞이면 즉시 400 에러 반환
   - Mainnet 배포 시 LocalTestnet 옵션(kubeconfigPath 등)이 있으면 무시하지 말고 경고 로그

3. **kubeconfig 파일 경로 검증 필수**
   - LocalTestnet 요청 시 kubeconfigPath가 없으면 즉시 400 에러
   - 백엔드가 직접 파일 존재 여부를 확인 (배포 시작 전)

4. **AWS credentials 조건부 처리**
   - LocalTestnet: AWS 필드 없어도 허용
   - Mainnet/Testnet: 기존 필수 검증 유지

5. **기존 테스트 100% 통과 유지**
   - 새 코드 추가 후 `go test ./...` 실행 필수
   - 기존 테스트가 깨지면 새 기능 추가 전에 반드시 수정

### 코딩 컨벤션

```go
// ✅ 올바른 패턴 — 명시적 LocalTestnet 분기
func (s *DeploymentService) executeDeployments(stack *StackEntity) error {
    switch stack.Network {
    case LocalTestnet:
        return s.deployLocalInfra(stack)
    default:
        return s.deployAWSInfra(stack)
    }
}

// ❌ 금지 패턴 — 암묵적 분기 (버그 위험)
func (s *DeploymentService) executeDeployments(stack *StackEntity) error {
    if stack.KubeconfigPath != "" {
        return s.deployLocalInfra(stack)
    }
    return s.deployAWSInfra(stack)
}
```

```go
// ✅ 올바른 패턴 — 에러 컨텍스트 포함
func (s *FundingService) GetStatus(stackID uuid.UUID) (*FundingStatusEntity, error) {
    status, err := s.repo.FindByStackID(stackID)
    if err != nil {
        return nil, fmt.Errorf("funding status not found for stack %s: %w", stackID, err)
    }
    return status, nil
}

// ❌ 금지 패턴 — 에러 컨텍스트 없음
func (s *FundingService) GetStatus(stackID uuid.UUID) (*FundingStatusEntity, error) {
    return s.repo.FindByStackID(stackID)
}
```

---

## 2. 디렉토리 구조 변경

```
trh-backend/
├── pkg/
│   ├── domain/entities/
│   │   ├── enums.go              ← LocalTestnet 추가
│   │   ├── stack.go              ← KubeconfigPath, SeedPhrase 필드 추가
│   │   └── funding.go            ← 신규
│   ├── constants/
│   │   └── deployment.go         ← DeployLocalInfra 상수 추가
│   ├── api/
│   │   ├── handlers/thanos/
│   │   │   ├── base.go           ← validate-deployment 수정
│   │   │   ├── deployment.go     ← AWS optional 처리
│   │   │   └── funding.go        ← 신규
│   │   ├── dtos/
│   │   │   ├── thanos_dto.go     ← AWS 필드 optional화
│   │   │   └── funding_dto.go    ← 신규
│   │   └── routes/
│   │       └── routes.go         ← funding-status 라우트 추가
│   ├── services/thanos/
│   │   ├── deployment.go         ← LocalTestnet 분기 추가
│   │   ├── stack_lifecycle.go    ← LocalTestnet 스텝 생성 수정
│   │   └── funding_service.go    ← 신규
│   └── infrastructure/postgres/
│       └── migrations/           ← funding_statuses 테이블 추가

trh-sdk/
└── pkg/stacks/thanos/ (또는 commands/)
    └── local_deploy.go           ← 신규: DeployLocalInfrastructure()

trh-platform/
└── docker-compose.yml            ← kubeconfig 볼륨 마운트 추가
```

---

## 3. 테스트 요구사항

### Unit 테스트

```go
// 배포 분기 테스트
func TestDeploymentService_LocalTestnet_SkipsAWS(t *testing.T) {
    // LocalTestnet 요청 → deploy-local-infra 스텝만 생성
    // deploy-aws-infra 스텝 없음 검증
}

func TestDeploymentService_Testnet_UsesAWS(t *testing.T) {
    // 기존 Testnet → deploy-aws-infra 스텝 생성 검증 (회귀 테스트)
}

// 검증 테스트
func TestValidation_LocalTestnet_NoAWSRequired(t *testing.T) {
    // AWS credentials 없이 LocalTestnet 요청 → 200 OK
}

func TestValidation_LocalTestnet_NeedKubeconfig(t *testing.T) {
    // kubeconfigPath 없이 LocalTestnet 요청 → 400 Bad Request
}

// 펀딩 상태 테스트
func TestFundingService_GetStatus_ReturnsFourEOAs(t *testing.T) {
    // 4개 EOA 주소 + 잔액 + fulfilled 반환 검증
}
```

### 통합 테스트 (kind 클러스터)

```bash
# Phase 2 완료 후 실행
KUBECONFIG=/tmp/trh-test.kubeconfig \
go test -v -tags=integration -timeout=180s ./...

# 검증 항목:
# - L2 노드 Pod Running 상태
# - L2 RPC 응답 확인 (eth_blockNumber)
# - 기존 unit 테스트 100% 통과
```

---

## 4. API 명세 추가

### 신규 엔드포인트

```
GET /api/v1/stacks/thanos/:id/funding-status
Authorization: Bearer {token}
Response: FundingStatusResponse
```

### 수정되는 엔드포인트

```
POST /api/v1/stacks/thanos/validate-deployment
- LocalTestnet 시 awsAccessKey, awsSecretAccessKey, awsRegion 없어도 통과
- LocalTestnet 시 kubeconfigPath 필수

POST /api/v1/stacks/thanos
- network: "local_testnet" 허용
- LocalTestnet 시 AWS 필드 optional
- LocalTestnet 시 kubeconfigPath 필드 추가
```

---

## 5. 금지 사항

- Mainnet 배포 로직 수정 — LocalTestnet 분기만 추가
- AWS SDK 호출을 LocalTestnet 코드 경로에 포함 — 완전히 격리
- kubeconfig 파일 내용을 로그에 출력 — 보안 위반
- seed_phrase를 평문으로 DB 저장 — 반드시 암호화
- 기존 E2E 테스트 수정 없이 깨뜨리기 — 항상 수정 후 통과 확인
