# ADR ③: AWS 자격증명 무상태화 (평문 저장 제거)

```
Status: Draft
Date: 2026-04-14
Owner: trh-platform team
Relates-to: trh-sdk/docs/design/temporary-credentials.md (ADR ②)
Tracked-by: trh-platform/docs/design/preset-aws-rollout.md
```

---

## Context

`pkg/services/thanos/stack_lifecycle.go:26`:

```go
config, err := json.Marshal(request)
```

`request`는 `dtos.DeployThanosRequest`이며, 여기에는 `AwsAccessKey`, `AwsSecretAccessKey`가 포함된다. 이 JSON이 그대로 `StackEntity.Config` 컬럼(JSONB)에 저장된다.

**현재 동작:**
- 사용자가 입력한 AWS 자격증명이 PostgreSQL `stack_entities.config` 컬럼에 Base64 없이 평문 JSON으로 저장된다.
- `stack_lifecycle.go:314-316`, `L394-396`: 배포 재시작/재개 시 `json.Unmarshal(stack.Config, &stackConfig)`로 꺼내 다시 사용한다.
- `L411-412`: 꺼낸 creds를 `NewThanosSDKClient`에 전달한다.

**위험:**
1. **DB 덤프 = AWS 권한 노출**: 백업 파일, DB 스냅샷, `pg_dump` 아카이브에 live AWS 자격증명이 그대로 들어간다.
2. **로그 유출**: ORM / query 로거가 INSERT 내용을 찍을 경우 creds가 로그에 남는다.
3. **감사 불가**: config JSON에 key가 있는 채로 감사 로그를 남기면 key가 audit trail에 노출된다.
4. **SSO 임시 자격증명의 경우**: SessionToken까지 저장 → 만료 후 무의미하지만 발급 당시 권한 범위가 DB에 남는다.
5. **컴플라이언스**: AWS Well-Architected, SOC2, GDPR 등 어느 프레임워크에서도 자격증명을 DB에 plaintext 저장하는 것을 허용하지 않는다.

---

## Decision

**AWS 자격증명은 절대 persist하지 않는다.** 각 요청마다 Electron이 HTTP body로 전달하고, backend는 in-memory로만 보관, 배포 태스크 종료 시 즉시 evict한다.

### 변경 1: `DeployThanosRequest` config 저장 시 creds 제거

`stack_lifecycle.go:26`에서 전체 request를 marshal하는 대신, creds를 제외한 `PersistedConfig` 서브셋만 저장한다.

```go
// 변경 전
config, err := json.Marshal(request)

// 변경 후
type PersistedConfig struct {
    ChainName                 string
    Network                   string
    InfraProvider             string
    L2BlockTime               int
    ChallengePerio            int
    BatchSubmissionFrequency  int
    OutputRootFrequency       int
    // ... non-credential chain params
    // AwsAccessKey     → 제거
    // AwsSecretAccessKey → 제거
    // AwsSessionToken  → 추가하지 않음
}
cfg := PersistedConfig{...} // request에서 creds 필드 제외 복사
config, err := json.Marshal(cfg)
```

### 변경 2: 배포 재개 시 creds 재요청

현재 `stack_lifecycle.go:314–412`는 `stack.Config`에서 creds를 꺼내 재배포에 사용한다. creds가 없어지면 **재개 시 creds를 외부에서 재주입**해야 한다.

**재주입 방식 — 권장: 무상태 reject (stateless)**

```
[Electron] POST /api/v1/stacks/{id}/resume
           Body: { awsAccessKey, awsSecretAccessKey, awsSessionToken, awsRegion }

[Backend]  재개 태스크 컨텍스트에만 in-memory 보관 → 태스크 완료/실패 시 evict
```

대안으로 WebSocket/SSE push(Electron이 만료 감지 → 자동 재전송) 방식도 가능하지만, backend 복잡도가 높아진다. **무상태 reject 방식을 권장**: backend가 creds 없는 재개 요청을 `401 Credentials required`로 거부 → Electron UI가 "재인증 필요" 모달 표시 → 사용자가 creds 재입력 또는 SSO 재로그인 → resume 재시도.

### 변경 3: Task context — in-memory 전용

`pkg/services/task_manager.go`의 태스크 컨텍스트(goroutine 생명주기)에서만 creds를 보관한다.

```go
type DeployTaskContext struct {
    StackID    uuid.UUID
    AwsConfig  *sdk.AWSConfig // goroutine 생명주기 내에만 존재
    // ...
}

// 태스크 완료/실패 시
func (t *DeployTaskContext) cleanup() {
    if t.AwsConfig != nil {
        t.AwsConfig.AccessKey = ""
        t.AwsConfig.SecretKey = ""
        t.AwsConfig.SessionToken = ""
        t.AwsConfig = nil
    }
}
```

### 변경 4: `StackEntity` 스키마 — creds 필드 영구 제거

`StackEntity.Config` JSONB 컬럼에서 creds 필드가 이미 없어지므로, 기존 row에 있을 수 있는 creds를 정리하는 마이그레이션 추가.

```sql
-- goose Up
UPDATE stack_entities
SET config = config
    - 'awsAccessKey'
    - 'awsSecretAccessKey'
    - 'awsSessionToken'
WHERE config ? 'awsAccessKey'
   OR config ? 'awsSecretAccessKey';

-- goose Down
-- (intentionally empty — credential removal is irreversible by design)
```

### 변경 5: 감사 로그

자격증명 자체는 절대 로그에 남기지 않는다. 대신:

```go
logger.Info("AWS deploy initiated",
    zap.String("stackId", stackId.String()),
    zap.String("awsRegion", cfg.Region),
    zap.String("awsAccount", awsAccountId), // STS GetCallerIdentity에서 얻은 account ID만
    zap.String("iamRole", assumedRoleArn),  // role ARN만
    zap.Time("credExpiration", *cfg.Expiration), // 만료 시간만 (SSO 경우)
)
```

---

## Consequences

- **Good**: DB 덤프/백업에 AWS 자격증명이 절대 포함되지 않음 — 보안 위험 완전 제거.
- **Good**: SSO 임시 자격증명 모델과 자연스럽게 정합 — TTL이 짧아 어차피 persist해도 의미 없음.
- **Good**: 감사 로그에 account ID / role ARN만 남아 컴플라이언스 친화적.
- **Trade-off**: 장시간 배포 중 SSO 세션 만료 시 사용자가 재인증해야 함. ADR ②의 `Expiration` 필드 + Electron의 refresh 로직(plan 항목 1)이 이 빈도를 최소화.
- **Migration**: 기존 `stack_entities.config`에 creds가 있는 row는 goose 마이그레이션으로 일괄 제거.

---

## Alternatives considered

- **KMS 암호화 저장**: DB에 KMS-encrypted ciphertext를 저장. 구현 복잡도 대비 이점이 적음 — SSO temp creds는 어차피 TTL이 짧아 저장 자체가 불필요. 장기 IAM key에만 의미 있으나, 장기 key 사용 자체를 deprecated으로 가는 방향이 옳음.
- **Electron이 매 poll마다 creds를 헤더로 전달**: 기존 WebSocket 연결을 활용해 진행 중인 태스크에 creds를 갱신 push. 가능하나 backend가 creds를 "구독"하는 방식이 되어 무상태 원칙에 반함.

---

## Implementation checklist

- [ ] `dtos/thanos.go`에 `PersistedConfig` 서브타입 정의 (creds 필드 제외)
- [ ] `stack_lifecycle.go:26` — `json.Marshal(request)` → `json.Marshal(PersistedConfig{...})`로 교체
- [ ] `stack_lifecycle.go:314–412` — Config에서 creds 읽는 경로 제거, 재개 시 request body에서만 수용
- [ ] `POST /api/v1/stacks/{id}/resume` 엔드포인트 추가 (creds를 body로 받아 task context에만 전달)
- [ ] `pkg/services/task_manager.go` — `DeployTaskContext.cleanup()` 구현, 태스크 종료 시 호출
- [ ] goose 마이그레이션 작성 및 테스트
- [ ] `NewThanosSDKClient` 호출 경로에서 `cfg.AwsSessionToken` 전달 (ADR ② 완료 후 연동)
- [ ] 감사 로그 (`account ID`, `role ARN`, `credExpiration`) 추가, key/secret 로그 제거
- [ ] 단위 테스트: `PersistedConfig` marshal 결과에 credential 필드 없음 검증
- [ ] `Status: Accepted` 로 업데이트
- [ ] 구현 PR merge 후 `Status: Shipped` + trh-wiki gap #2 제거
