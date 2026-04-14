# ADR ④: Preset 모듈 자동 Install (AWS 경로)

```
Status: Draft
Date: 2026-04-14
Owner: trh-platform team
Relates-to:
  - tokamak-thanos-stack/docs/design/preset-helm-values-matrix.md (ADR ①)
  - trh-backend/docs/design/credential-storage.md (ADR ③)
Tracked-by: trh-platform/docs/design/preset-aws-rollout.md
```

---

## Context

`trh-sdk/pkg/stacks/thanos/deploy_chain.go:561–590` — preset에 따라 조건부 모듈 install을 수행하는 현재 코드:

```go
if enabled := presetModules["monitoring"]; enabled {
    t.logger.Info("ℹ️  Monitoring is included in your preset. Run 'trh install monitoring' to configure and deploy it.")
}
if enabled := presetModules["blockExplorer"]; enabled {
    t.logger.Info("ℹ️  Block Explorer is included in your preset. Run 'trh install block-explorer' to configure and deploy it.")
}
if enabled := presetModules["crossTrade"]; enabled {
    t.logger.Info("ℹ️  Cross-Chain Trade is included in your preset. Run 'trh install cross-trade' to configure and deploy it.")
}
```

`uptimeService`는 자동으로 `InstallUptimeService`를 호출하는 반면, `monitoring`/`blockExplorer`/`crossTrade`는 로그만 남기고 **사용자에게 수동 `trh install <X>` 명령어를 요구**한다.

**Local Docker 경로와의 비대칭:**

| 모듈 | AWS | Local Docker |
|------|-----|-------------|
| uptimeService | ✅ 자동 (SDK) | ✅ 자동 (compose) |
| monitoring | ❌ 수동 `trh install monitoring` | ✅ 자동 (compose profile) |
| blockExplorer | ❌ 수동 `trh install block-explorer` | ✅ 자동 (compose) |
| crossTrade | ❌ 수동 `trh install cross-trade` | ✅ 자동 (compose + cross_trade_local.go) |

이 비대칭이 `ec2-deploy.md` **gap #7**이다. Electron 앱 사용자는 preset 선택으로 모든 모듈이 자동 배포되기를 기대하지만, AWS 경로는 배포 완료 후 CLI를 별도 실행해야 한다.

trh-sdk가 제공하는 함수:
- `InstallBlockExplorer(ctx, *InstallBlockExplorerInput) (string, error)`
- `InstallMonitoring(ctx, *MonitoringConfig) (*MonitoringInfo, error)`
- `InstallCrossTrade(ctx, ...)` (cross_trade.go)
- `InstallUptimeService(ctx, *UptimeServiceConfig) (string, error)` — 이미 자동화됨

---

## Decision

**trh-backend의 배포 오케스트레이터에 `installPresetModules(preset, stackCtx)` 단계를 추가한다.** 메인 L2 인프라 배포(`DeployAWSInfrastructure`) 완료 후, preset 매핑에 따라 필요한 모듈을 순차 자동 install한다. 사용자는 수동 CLI 명령어를 실행할 필요가 없다.

### 모듈 install 순서 (의존성 그래프)

```
1. L2 인프라 배포 완료 (op-geth / op-node / op-batcher / op-proposer Running)
   └─ L2 RPC URL 확정 (ingress 주소 발견)
        │
        ├─ 2. Bridge install (이미 자동, 항상)
        │
        ├─ 3. Block Explorer install
        │     의존: L2 RPC healthy (eth_chainId 응답)
        │
        ├─ 4. Uptime Service install (이미 자동화)
        │     의존: L2 RPC healthy
        │
        ├─ 5. Monitoring install
        │     의존: Block Explorer healthy (Explorer URL 200 응답)
        │
        └─ 6. CrossTrade install
              의존: L1 Deposit Tx 1건 성공
              (CrossTrade bridge는 L1↔L2 메시지 릴레이가 필요하므로 L1 사이드가 준비 후)
```

DRB / AA Paymaster는 ADR ①(Helm values 분기)에서 Helm chart level에서 preset별 enable/disable로 처리 → 별도 `Install*` 호출 불필요. Backup은 Full 전용이며 EFS + backup manager chart로 처리.

### backend 오케스트레이터 변경

`pkg/services/thanos/deployment.go`의 `executeDeployments` (L435) 내부 또는 직후에 `installPresetModules` 단계 추가:

```go
// deployment.go (개념 코드)
func (s *ThanosDeploymentService) installPresetModules(
    ctx context.Context,
    preset string,
    stackClient *sdk.ThanosStack,
    stackCtx *DeployTaskContext,
) error {
    presetModules := constants.PresetModules[preset]

    // Step 1: Block Explorer (L2 RPC healthy 후)
    if presetModules["blockExplorer"] {
        if err := s.waitAndInstallBlockExplorer(ctx, stackClient, stackCtx); err != nil {
            s.emitModuleProgress(stackCtx.StackID, "blockExplorer", ModuleStatusFailed, err)
            // 부분 성공 허용 — 로그만 남기고 계속 진행
            logger.Error("block explorer install failed", zap.Error(err))
        } else {
            s.emitModuleProgress(stackCtx.StackID, "blockExplorer", ModuleStatusDone, nil)
        }
    }

    // Step 2: Monitoring (Block Explorer healthy 후)
    if presetModules["monitoring"] {
        if err := s.waitAndInstallMonitoring(ctx, stackClient, stackCtx); err != nil {
            s.emitModuleProgress(stackCtx.StackID, "monitoring", ModuleStatusFailed, err)
            logger.Error("monitoring install failed", zap.Error(err))
        } else {
            s.emitModuleProgress(stackCtx.StackID, "monitoring", ModuleStatusDone, nil)
        }
    }

    // Step 3: CrossTrade (L1 Deposit Tx 후)
    if presetModules["crossTrade"] {
        if err := s.waitAndInstallCrossTrade(ctx, stackClient, stackCtx); err != nil {
            s.emitModuleProgress(stackCtx.StackID, "crossTrade", ModuleStatusFailed, err)
            logger.Error("cross-trade install failed", zap.Error(err))
        } else {
            s.emitModuleProgress(stackCtx.StackID, "crossTrade", ModuleStatusDone, nil)
        }
    }

    return nil // 개별 모듈 실패는 부분 성공으로 처리
}
```

### 진행 상태 스트리밍 — platform-ui 계약

기존 배포 진행 WebSocket/SSE 채널에 `moduleProgress` 이벤트 추가:

```json
{
  "event": "moduleProgress",
  "moduleKey": "blockExplorer",
  "status": "installing" | "done" | "failed",
  "message": "Installing Block Explorer...",
  "timestamp": "2026-04-14T10:30:00Z"
}
```

platform-ui는 이 이벤트를 받아 배포 진행 화면의 서브 스텝으로 렌더링한다. 기존 L2 인프라 진행 표시 컴포넌트에 모듈별 서브 스텝 항목을 추가한다.

### 실패 정책 — 부분 성공 허용

- 개별 모듈 install 실패 → 전체 배포 실패로 처리하지 않는다.
- L2 코어(op-geth/op-node/batcher/proposer)가 살아있으면 배포 자체는 성공.
- 실패한 모듈은 `ModuleStatusFailed`로 기록, 플랫폼 UI에서 재시도 버튼 노출.
- `DeploymentEntity.Status = completed_with_warnings` (또는 기존 `completed` + `IntegrationEntity.Status = failed`로 모듈별 상태 추적).

### 재시도 API

```
POST /api/v1/stacks/{id}/modules/{moduleKey}/install
Body: { awsAccessKey, awsSecretAccessKey, awsSessionToken, awsRegion }  ← ADR ③ stateless 방식
```

이 엔드포인트는 idempotent — 이미 설치된 모듈에 대해 호출하면 `409 Already installed` 또는 재설치(uninstall → install). 재시도 의미론은 모듈별 install 함수의 idempotency에 따른다.

---

## Consequences

- **Good**: Electron 앱 사용자가 preset 선택 → 단일 클릭으로 L2 + 전 모듈 배포 완료. 수동 CLI 없음.
- **Good**: Local Docker 경로와 동일한 사용자 경험 — 비대칭 해소.
- **Good**: 모듈별 부분 성공 허용으로 네트워크 일시 장애에 강건.
- **Trade-off**: 모듈 install은 L2 인프라 배포 이후 순차 실행 → 전체 소요 시간 증가. General preset ~13분, DeFi ~18분, Gaming ~20분, Full ~25분 기준 (presets.md 참조).
- **Trade-off**: trh-sdk의 `InstallMonitoring`, `InstallBlockExplorer`, `InstallCrossTrade` 함수들이 필요한 입력값(config struct)을 backend가 구성해야 함 — 각 함수의 input 필드 매핑 분석 필요.
- **Migration**: 기존 배포된 Stack에서 모듈 미설치 상태는 `IntegrationEntity.Status = failed`로 마킹, 재시도 API로 수동 트리거 가능.

---

## Alternatives considered

- **trh-sdk에서 직접 자동화**: `deploy_chain.go:561–590`의 로그 메시지를 실제 install 호출로 교체. SDK가 CLI(`trh deploy`) 실행 시에도 자동 install하게 됨. 그러나 CLI 사용자는 install 전 설정(grafana URL, alert rules 등)을 직접 입력해야 하므로 interactive prompt 없는 자동화가 부적절. SDK는 primitive를 제공하고 backend가 오케스트레이션하는 것이 계층 분리에 맞음.
- **배포 완료 후 별도 background job**: `cron` 또는 별도 goroutine으로 모듈 install. 배포 성공 이벤트에 커플링이 느슨해져 retry 복잡도 증가. 배포 태스크 컨텍스트 안에서 처리하는 것이 creds 생명주기 관리(ADR ③)와 자연스럽게 맞음.

---

## Implementation checklist

- [ ] `pkg/services/thanos/deployment.go`에 `installPresetModules` 함수 구현
- [ ] `constants.PresetModules` 맵을 backend도 참조할 수 있도록 trh-sdk export 확인 (또는 backend 내부 정의)
- [ ] Block Explorer install 전 L2 RPC healthy 대기 (`waitForRpcReady` 헬퍼)
- [ ] Monitoring install 전 Block Explorer URL 200 응답 대기
- [ ] CrossTrade install 전 L1 Deposit Tx 확인 로직 (또는 fixed delay + retry 방식)
- [ ] `moduleProgress` 이벤트 정의 + SSE/WebSocket 채널에 emit
- [ ] `POST /api/v1/stacks/{id}/modules/{moduleKey}/install` 엔드포인트 추가 (ADR ③ stateless creds 방식)
- [ ] `IntegrationEntity.Status` 모델에 `failed` / `installing` / `completed` 상태 확인 또는 추가
- [ ] platform-ui 팀과 `moduleProgress` 이벤트 계약 합의 (platform-ui별도 작업)
- [ ] trh-sdk `InstallBlockExplorer` / `InstallMonitoring` / `InstallCrossTrade` input struct 매핑 분석
- [ ] ADR ① (Helm values 분기) 완료 후 DRB / AA Paymaster install 경로 불필요함 재확인
- [ ] 단위 테스트: preset별 `installPresetModules` 호출 모듈 목록 검증
- [ ] 통합 테스트: mock SDK로 순서 의존성 검증 (Explorer → Monitoring 순서 보장)
- [ ] `Status: Accepted` 로 업데이트
- [ ] 구현 PR merge 후 `Status: Shipped` + trh-wiki gap #7 제거
