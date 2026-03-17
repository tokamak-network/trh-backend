# Phase 분리 계획 — LocalTestnet 배포

**버전**: 1.0
**작성일**: 2026-03-10
**총 기간**: 3주

---

## 전체 로드맵

```
Phase 1 (1주)          Phase 2 (1주)             Phase 3 (1주)
trh-backend 분기       trh-sdk 로컬 배포          키 생성 + 펀딩 API
────────────           ──────────────────         ─────────────────
LocalTestnet enum      DeployLocalInfra()         FundingStatus 엔티티
deploy-local-infra     kind + Helm 배포           /funding-status API
AWS optional           SDK 버전 범프              L1 잔액 폴링
```

---

## Phase 1 — trh-backend 핵심 분기 (1주)

**목표**: `network: local_testnet` 요청이 들어왔을 때 기존 AWS 로직을 건너뛰고 새 스텝으로 분기

**변경 레포**: `trh-backend`

### 체크리스트

```
pkg/domain/entities/enums.go
- [ ] LocalTestnet DeploymentNetwork 상수 추가

pkg/constants/deployment.go
- [ ] DeployLocalInfra = "deploy-local-infra" 상수 추가

pkg/domain/entities/stack.go
- [ ] KubeconfigPath string 필드 추가
- [ ] SeedPhrase string 필드 추가

pkg/api/dtos/thanos_dto.go
- [ ] AWSAccessKey, AWSSecretKey, AWSRegion → *string (optional)
- [ ] KubeconfigPath *string 필드 추가
- [ ] SeedPhrase *string 필드 추가

pkg/api/handlers/thanos/base.go (validate-deployment)
- [ ] LocalTestnet 시 AWS credentials 없어도 통과하도록 수정
- [ ] KubeconfigPath 필수 검증 (LocalTestnet 시)

pkg/services/thanos/stack_lifecycle.go
- [ ] LocalTestnet 시 deploy-local-infra 스텝 생성
- [ ] LocalTestnet 시 deploy-aws-infra 스텝 생성 안 함

pkg/services/thanos/deployment.go
- [ ] executeDeployments()에 LocalTestnet 분기 추가
- [ ] LocalTestnet → thanos.DeployLocalInfrastructure() 호출 (Phase 2에서 구현)
- [ ] Mainnet/Testnet → 기존 DeployAWSInfrastructure() 유지

DB 마이그레이션
- [ ] stacks 테이블에 kubeconfig_path, seed_phrase 컬럼 추가
```

### 완료 기준

```bash
# LocalTestnet으로 배포 요청 시 deploy-local-infra 스텝 생성 확인
POST /api/v1/stacks/thanos
{
  "network": "local_testnet",
  "kubeconfigPath": "/tmp/trh-test.kubeconfig",
  "chainName": "test-chain",
  "l1RpcUrl": "https://sepolia.rpc..."
  # awsAccessKey 없어도 통과
}
→ 201 Created, step: "deploy-local-infra"
```

---

## Phase 2 — trh-sdk DeployLocalInfrastructure (1주)

**목표**: kind 클러스터에 L2 노드를 Helm으로 배포하는 로직 구현

**변경 레포**: `trh-sdk` (feat/runner-k8s-native 브랜치)

### 체크리스트

```
pkg/stacks/thanos/ (또는 commands/)
- [ ] DeployLocalInfrastructure(ctx, config) error 함수 추가
- [ ] kubeconfig 경로 받아 K8sRunner + HelmRunner 초기화
- [ ] 네임스페이스 생성 (EnsureNamespace)
- [ ] Helm chart 배포: op-geth, op-node, op-batcher, op-proposer
- [ ] 배포 완료 대기 (Pod Running 상태 확인)
- [ ] L2 RPC URL 반환

trh-backend (go.mod 업데이트)
- [ ] trh-sdk 버전 범프 (새 버전 태그 필요)
- [ ] go.mod에서 신규 버전으로 업데이트
- [ ] thanos.DeployLocalInfrastructure() 호출 코드 연결
```

### 활용 가능한 기존 코드

```go
// 이미 구현 완료 (feat/runner-k8s-native)
runner.New(RunnerConfig{
    UseNative:      true,
    KubeconfigPath: kubeconfigPath,
})
// → K8sRunner, HelmRunner 모두 사용 가능
// → kind 클러스터 통합 테스트에서 이미 검증됨
```

### 완료 기준

```bash
# kind 클러스터에 L2 노드 배포 성공
KUBECONFIG=/tmp/trh-test.kubeconfig \
trh-sdk deploy --network local-testnet --chain-name test-chain

# Pod 상태 확인
kubectl --kubeconfig=/tmp/trh-test.kubeconfig get pods
# → op-geth, op-node, op-batcher, op-proposer Running
```

---

## Phase 3 — 키 생성 + 펀딩 API (1주)

**목표**: Seed → EOA 자동 파생 + L1 잔액 확인 API

**변경 레포**: `trh-backend`, `trh-platform`

### 체크리스트

```
trh-backend:

pkg/domain/entities/funding.go (신규)
- [ ] FundingStatusEntity 구조체 정의
- [ ] EOAFunding 구조체 정의

pkg/infrastructure/postgres/ (마이그레이션)
- [ ] funding_statuses 테이블 생성 마이그레이션

pkg/api/handlers/thanos/funding.go (신규)
- [ ] GET /stacks/thanos/:id/funding-status 핸들러

pkg/api/routes/
- [ ] funding-status 라우트 등록

pkg/services/thanos/funding_service.go (신규)
- [ ] L1 RPC로 4개 EOA 잔액 조회
- [ ] 필요 금액과 비교 → fulfilled 계산
- [ ] FundingStatus DB 업데이트

trh-platform:

docker-compose.yml
- [ ] backend 서비스에 kubeconfig 볼륨 마운트 추가
  volumes:
    - /tmp/trh-test.kubeconfig:/root/.kube/local-testnet.kubeconfig:ro
```

### 완료 기준

```bash
# 펀딩 상태 조회 API
GET /api/v1/stacks/thanos/{id}/funding-status
→ {
    "deployer":   { "address": "0x...", "requiredWei": "500000000000000000", "currentWei": "0", "fulfilled": false },
    "batcher":    { ... },
    "proposer":   { ... },
    "challenger": { ... },
    "allFulfilled": false
  }
```

---

## 마일스톤 요약

| 마일스톤 | 완료 시점 | 결과물 |
|---------|----------|-------|
| Phase 1 완료 | 1주차 | LocalTestnet 분기 로직 + 기존 테스트 100% 유지 |
| Phase 2 완료 | 2주차 | kind 클러스터에 실제 L2 배포 성공 |
| Phase 3 완료 | 3주차 | 펀딩 상태 API + trh-platform kubeconfig 마운트 |

---

## 리스크 및 대응

| 리스크 | 가능성 | 대응 |
|--------|-------|------|
| kind 내부에서 L2 노드 리소스 부족 | 중간 | 최소 8GB RAM 권장, requests/limits 조정 |
| Sepolia RPC rate limit | 중간 | Alchemy/Infura 무료 키 권장 가이드 제공 |
| trh-sdk 버전 관리 복잡성 | 낮음 | 로컬 replace directive 활용 개발 후 태그 |
| 기존 AWS 배포 회귀 | 낮음 | 분기 로직 단순화 + 기존 E2E 테스트 유지 |
