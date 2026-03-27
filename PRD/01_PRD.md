# PRD — LocalTestnet: 클라우드 없는 L2 로컬 배포

**버전**: 1.0
**작성일**: 2026-03-10
**프로젝트**: tokamak-network/trh-backend + trh-sdk + trh-platform

---

## 1. 배경 및 문제

TRH Platform은 현재 L2 롤업 배포 시 **AWS 필수 의존성**이 있다.

```
현재 배포 플로우:
  1. AWS Access Key 설정
  2. Terraform → AWS EKS 프로비저닝
  3. Helm → L2 노드 (op-geth, op-node, op-batcher, op-proposer)
```

### 사용자가 겪는 문제

| 단계 | 장벽 | 소요 시간 |
|------|------|---------|
| AWS 계정 생성 + IAM Key 발급 | 개발 경험 없으면 막힘 | 10~30분 |
| EKS 클러스터 프로비저닝 | 비용 발생, 실패 가능성 | 10~15분 |
| 전체 배포 | AWS 설정 오류로 반복 실패 | 2~3시간 |

**초급 개발자가 "L2 체인을 한번 만들어보고 싶다"는 목표를 달성하기 전에 AWS 장벽에서 포기한다.**

---

## 2. 목표

### 사용자 목표
- AWS 계정 없이 로컬 머신에서 L2 체인을 배포하고 실제 Sepolia L1에 연결해볼 수 있다
- 전체 배포 소요 시간: **~25분** (기존 2~3시간)

### 기술 목표
- `DeploymentNetwork`에 `LocalTestnet` 타입 추가 — 기존 코드 최소 변경
- `deploy-aws-infra` 스텝을 `deploy-local-infra` (kind + Helm)로 교체
- AWS credentials 없이 배포 가능하게
- 키 자동 생성 + 펀딩 상태 확인 API 제공

### 비목표
- Mainnet 로컬 배포 (지원 안 함)
- UI 변경 (trh-platform-ui) — 이번 범위 외
- Preset 시스템 — 별도 PRD로 관리

---

## 3. 타겟 사용자

| 타겟 | 설명 | 우선순위 |
|------|------|---------|
| **초급 개발자** | L2 배포 경험 없음. AWS 모름. 로컬 Docker는 사용 가능 | 🔴 1차 타겟 |
| 인프라 운영자 | 기존 AWS 배포 계속 사용 | 기존 플로우 유지 |
| AI 에이전트 | MCP를 통한 프로그래밍 방식 배포 | 장기 방향 |

---

## 4. 사용자 스토리

| ID | 스토리 | 수용 기준 |
|----|--------|----------|
| US-01 | 개발자가 AWS 없이 로컬에서 L2를 배포할 수 있다 | `network: local_testnet` + kubeconfig 경로만으로 배포 성공 |
| US-02 | L2가 실제 Sepolia에 연결된다 | L1 컨트랙트가 Sepolia에 배포되고, L2 체인이 Sepolia를 L1으로 사용 |
| US-03 | 배포 키가 자동으로 생성된다 | Seed 1개 → Deployer/Batcher/Proposer/Challenger 4개 자동 파생 |
| US-04 | 펀딩 상태를 API로 확인할 수 있다 | `/funding-status` 에서 4개 EOA 잔액 + 충족 여부 반환 |
| US-05 | 기존 Mainnet/Testnet 배포는 변경 없이 동작 | 기존 E2E 테스트 100% 통과 |

---

## 5. 핵심 설계

### 배포 모드 분기

```
network = "local_testnet"
  └── deploy-l1-contracts  → Sepolia L1에 컨트랙트 배포 (기존 동일)
  └── deploy-local-infra   → kubeconfig로 kind 연결 → Helm 배포 (신규)

network = "testnet" / "mainnet"
  └── deploy-l1-contracts  → 기존 동일
  └── deploy-aws-infra     → 기존 동일 (변경 없음)
```

### kind 클러스터 전제 조건

- 사용자가 kind 클러스터를 미리 생성하고 kubeconfig 경로를 입력
- 백엔드는 kubeconfig를 받아 Helm 배포만 실행
- Docker-in-Docker 이슈 없음

---

## 6. 성공 지표

| 지표 | 현재 | 목표 |
|------|------|------|
| 로컬 배포 시 AWS 필요 여부 | 필수 | 불필요 |
| 초급 개발자 배포 성공률 | ~30% | >80% |
| 로컬 배포 소요 시간 | 2~3시간 | ~25분 |
| 기존 배포 회귀 | — | 0건 |
