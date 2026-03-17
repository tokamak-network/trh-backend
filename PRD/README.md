# PRD — LocalTestnet: 클라우드 없는 L2 로컬 배포

**목표**: AWS 없이 kind + Sepolia로 L2를 로컬에서 배포
**대상 레포**: trh-backend, trh-sdk, trh-platform
**총 기간**: 3주

---

## 문서 목록

| 문서 | 내용 |
|------|------|
| [01_PRD.md](./01_PRD.md) | 배경, 목표, 타겟 사용자, 사용자 스토리 |
| [02_DATA_MODEL.md](./02_DATA_MODEL.md) | 엔티티 변경, 신규 테이블, API DTO |
| [03_PHASES.md](./03_PHASES.md) | Phase 1~3 체크리스트, 완료 기준, 리스크 |
| [04_PROJECT_SPEC.md](./04_PROJECT_SPEC.md) | AI 행동 규칙, 디렉토리 구조, 테스트 요구사항 |

---

## 핵심 변경 요약

```
Phase 1 (trh-backend): LocalTestnet enum + deploy-local-infra 분기
Phase 2 (trh-sdk):     DeployLocalInfrastructure() — kind + Helm
Phase 3 (trh-backend + trh-platform): 펀딩 API + kubeconfig 마운트
```

## 다음 시작 지점

```
trh-backend/pkg/domain/entities/enums.go
→ LocalTestnet 추가부터 시작
```
