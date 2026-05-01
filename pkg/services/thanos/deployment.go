package thanos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/constants"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/enum"
	"github.com/tokamak-network/trh-backend/pkg/services/thanos/integrations"
	"github.com/tokamak-network/trh-backend/pkg/services/thanos/presets"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	thanosSDKConstants "github.com/tokamak-network/trh-sdk/pkg/constants"
	thanosSDKStack "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	thanosSDKTypes "github.com/tokamak-network/trh-sdk/pkg/types"
	trhSDKUtils "github.com/tokamak-network/trh-sdk/pkg/utils"
	"go.uber.org/zap"
)

// New helper method to handle deployment logic
func (s *ThanosStackDeploymentService) deploy(ctx context.Context, stackId uuid.UUID) {
	err := s.executeDeployments(ctx, stackId)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			logger.Info("deployment cancelled", zap.String("stackId", stackId.String()))
			return
		}
		logger.Error("failed to deploy thanos stacks",
			zap.String("stackId", stackId.String()),
			zap.Error(err))

		// Update stacks status to failed
		updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusFailedToDeploy, err.Error())
		if updateErr != nil {
			logger.Error("failed to update stacks status",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
		}

		err = s.integrationRepo.UpdateIntegrationsStatusByStackID(
			stackId.String(),
			entities.DeploymentStatusFailed,
			[]entities.DeploymentStatus{entities.DeploymentStatusTerminated},
			[]string{enum.IntegrationTypeRegisterCandidate.String()},
		)
		if err != nil {
			logger.Error("failed to update integrations status", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}

		return
	}

	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack by id", zap.String("stackId", stackId.String()))
		return
	}

	// Update stacks status to active on success
	updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusDeployed, "")
	if updateErr != nil {
		logger.Error("failed to update stacks status",
			zap.String("stackId", stackId.String()),
			zap.Error(updateErr))
	}

	config, err := json.Marshal(stack.Config)
	if err != nil {
		logger.Error("failed to marshal stack config", zap.Error(err))
		return
	}
	var stackConfig dtos.DeployThanosRequest
	if err := json.Unmarshal(config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.Error(err))
		return
	}

	logPath := utils.GetLogPath(stack.ID, "information")
	sdkClient, err := thanos.NewThanosSDKClient(
		ctx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
		stackConfig.RegisterCandidate,
		stackConfig.AwsAccessKey,
		stackConfig.AwsSecretAccessKey,
		stackConfig.AwsRegion,
	)
	if err != nil {
		logger.Error("failed to create thanos sdk client",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	// Get chain information — local infra deployments don't have K8s, so build
	// chain info directly from well-known localhost ports.
	var chainInformation *thanosSDKTypes.ChainInformation
	if stackConfig.InfraProvider == "local" {
		chainInformation = thanos.BuildLocalChainInformation(stack.DeploymentPath)
	} else {
		chainInformation, err = thanos.ShowChainInformation(ctx, sdkClient)
		if err != nil || chainInformation == nil {
			logger.Error("failed to show chain information", zap.Error(err))
			return
		}
	}

	var layer1Name string
	if string(stack.Network) == "mainnet" {
		layer1Name = "Ethereum"
	} else {
		layer1Name = "Ethereum Sepolia"
	}

	err = s.stackRepo.UpdateMetadata(stackId.String(), &entities.StackMetadata{
		Layer1:          layer1Name,
		Layer2:          "Thanos Stack",
		L1ChainId:       chainInformation.L1ChainID,
		L2RpcUrl:        chainInformation.L2RpcUrl,
		L2ChainId:       chainInformation.L2ChainID,
		BridgeUrl:       chainInformation.BridgeUrl,
		ExplorerUrl:     chainInformation.BlockExplorer,
		RollupConfigUrl: chainInformation.RollupFilePath,
		MonitoringUrl:   chainInformation.MonitoringUrl,
	})
	if err != nil {
		logger.Error("failed to update stack metadata", zap.Error(err))
		return
	}

	bridgeUrl := chainInformation.BridgeUrl

	// bridgeIntegration
	bridgeIntegration, err := s.integrationRepo.GetIntegration(stackId.String(), enum.IntegrationTypeBridge.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return
	}

	if bridgeIntegration == nil {
		logger.Error("bridge integration not found", zap.String("plugin", enum.IntegrationTypeBridge.String()))
		return
	}

	metadata := map[string]string{
		"url": bridgeUrl,
	}

	bytes, err := json.Marshal(metadata)
	if err != nil {
		logger.Error("failed to marshal bridge metadata", zap.Error(err))
		return
	}

	err = s.integrationRepo.UpdateMetadataAfterInstalled(
		bridgeIntegration.ID.String(),
		entities.IntegrationInfo(bytes),
	)

	if err != nil {
		logger.Error("failed to create integration", zap.Error(err))
		return
	}

	if stackConfig.RegisterCandidate && stackConfig.RegisterCandidateParams != nil {
		registerCandidateIntegration, err := s.integrationRepo.GetIntegration(stackId.String(), enum.IntegrationTypeRegisterCandidate.String())
		if err != nil {
			logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(err))
			return
		}

		if registerCandidateIntegration == nil {
			logger.Error("register candidate integration not found", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()))
			return
		}

		registerCandidateInfo, err := thanos.GetRegisterCandidatesInfo(ctx, sdkClient, stackConfig.RegisterCandidateParams)
		if err != nil {
			logger.Error("failed to get register candidate info", zap.Error(err))
			return
		}

		bytes, err := json.Marshal(registerCandidateInfo)
		if err != nil {
			logger.Error("failed to marshal register candidate info", zap.Error(err))
			return
		}

		err = s.integrationRepo.UpdateMetadataAfterInstalled(
			registerCandidateIntegration.ID.String(),
			bytes,
		)

		if err != nil {
			logger.Error("failed to update register candidate integration metadata", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(err))
			return
		}
	}

	// Auto-install preset modules that don't require user configuration.
	// For local infra, mark block-explorer / monitoring / uptimeService / DRB as
	// installed directly (they run as Docker Compose profiles; no user config needed).
	if stackConfig.PresetID != "" {
		presetSvc := presets.NewService()
		def, err := presetSvc.GetByID(stackConfig.PresetID)
		if err != nil {
			logger.Error("failed to get preset definition for auto-install", zap.String("presetId", stackConfig.PresetID), zap.Error(err))
		} else if stackConfig.InfraProvider == "local" {
			localIntegrationURLs := map[string]string{
				enum.IntegrationTypeBlockExplorer.String(): chainInformation.BlockExplorer,
				enum.IntegrationTypeMonitoring.String():    chainInformation.MonitoringUrl,
				enum.IntegrationTypeUptimeService.String(): "http://localhost:3003",
				enum.IntegrationTypeDRB.String():           "http://localhost:9600",
			}
			for intType, url := range localIntegrationURLs {
				moduleKey := intType
				// map integration type names to preset module keys
				switch intType {
				case enum.IntegrationTypeBlockExplorer.String():
					moduleKey = "blockExplorer"
				case enum.IntegrationTypeMonitoring.String():
					moduleKey = "monitoring"
				case enum.IntegrationTypeUptimeService.String():
					moduleKey = "uptimeService"
				case enum.IntegrationTypeDRB.String():
					moduleKey = "drb"
				}
				enabled, ok := def.Modules[moduleKey]
				if !ok || !enabled {
					continue
				}
				integration, err := s.integrationRepo.GetIntegration(stackId.String(), intType)
				if err != nil || integration == nil {
					logger.Error("failed to get integration for local auto-install", zap.String("type", intType), zap.Error(err))
					continue
				}
				metaBytes, _ := json.Marshal(map[string]string{"url": url})
				if err := s.integrationRepo.UpdateMetadataAfterInstalled(integration.ID.String(), entities.IntegrationInfo(metaBytes)); err != nil {
					logger.Error("failed to mark local integration as installed", zap.String("type", intType), zap.Error(err))
				}
			}

			// CrossTrade auto-install for local DeFi/Full preset.
			// Unlike other modules (which just mark a URL), CrossTrade requires SDK contract
			// deployment via L1 OptimismPortal depositTransaction calls.
			if enabled, ok := def.Modules["crossTrade"]; ok && enabled {
				crossTradeIntegration, getErr := s.integrationRepo.GetIntegration(stackId.String(), enum.IntegrationTypeCrossTrade.String())
				if getErr != nil || crossTradeIntegration == nil {
					logger.Error("failed to get crossTrade integration for local auto-install",
						zap.String("stackId", stackId.String()), zap.Error(getErr))
				} else {
					crossTradeOutput, ctErr := s.autoInstallCrossTradeLocal(ctx, stack, &stackConfig, chainInformation)
					if ctErr != nil {
						logger.Error("failed to auto-install CrossTrade local",
							zap.String("stackId", stackId.String()), zap.Error(ctErr))
						if updateErr := s.integrationRepo.UpdateIntegrationStatusWithReason(
							crossTradeIntegration.ID.String(),
							entities.DeploymentStatusFailed,
							ctErr.Error(),
						); updateErr != nil {
							logger.Error("failed to update crossTrade integration status",
								zap.String("stackId", stackId.String()), zap.Error(updateErr))
						}
					} else {
						// Read L1 bridge addresses from deploy.json for L2toL2 setChainInfo 7-param call.
						// trhSDKUtils.ReadDeployementConfigFromJSONFile returns the {l1ChainID}-deploy.json contracts.
						l1Contracts, contractsErr := trhSDKUtils.ReadDeployementConfigFromJSONFile(
							stack.DeploymentPath,
							uint64(chainInformation.L1ChainID),
						)
						if contractsErr != nil {
							logger.Error("failed to read L1 contracts for CrossTrade registration",
								zap.String("stackId", stackId.String()), zap.Error(contractsErr))
							if updateErr := s.integrationRepo.UpdateIntegrationStatusWithReason(
								crossTradeIntegration.ID.String(),
								entities.DeploymentStatusFailed,
								fmt.Sprintf("read L1 contracts: %v", contractsErr),
							); updateErr != nil {
								logger.Error("failed to update crossTrade integration status",
									zap.String("stackId", stackId.String()), zap.Error(updateErr))
							}
							return
						}
						l1StandardBridge := l1Contracts.L1StandardBridgeProxy
						l1USDCBridge := l1Contracts.L1UsdcBridgeProxy
						l1CrossDomainMessenger := l1Contracts.L1CrossDomainMessengerProxy
						// BE-04, BE-05, BE-06: L1 CrossTrade 컨트랙트에 새 L2 등록 (setChainInfo x2, max 3 retries)
						regInput := &integrations.CrossTradeL1RegistrationInput{
							L1RPCURL:               stackConfig.L1RpcUrl,
							L1ChainID:              uint64(chainInformation.L1ChainID),
							L2ChainID:              uint64(chainInformation.L2ChainID),
							DeployerPrivKey:        stackConfig.AdminAccount,
							L2CrossTradeProxy:      crossTradeOutput.L2CrossTradeProxy,
							L2toL2CrossTradeProxy:  crossTradeOutput.L2toL2CrossTradeProxy,
							L1StandardBridge:       l1StandardBridge,
							L1USDCBridge:           l1USDCBridge,
							L1CrossDomainMessenger: l1CrossDomainMessenger,
						}
						regOutput, regErr := integrations.RegisterCrossTradeL2(ctx, regInput, 3)
						if regErr != nil {
							// D-01: L2 컨트랙트는 배포됐으나 L1 등록 실패 → CrossTrade integration만 failed 표시
							// crossTradeOutput은 보존되며 L2 배포 성공은 덮어쓰지 않음
							logger.Error("CrossTrade L1 registration (setChainInfo) failed",
								zap.String("stackId", stackId.String()), zap.Error(regErr))
							if updateErr := s.integrationRepo.UpdateIntegrationStatusWithReason(
								crossTradeIntegration.ID.String(),
								entities.DeploymentStatusFailed,
								regErr.Error(),
							); updateErr != nil {
								logger.Error("failed to update crossTrade integration status after L1 registration failure",
									zap.String("stackId", stackId.String()), zap.Error(updateErr))
							}
						} else {
							// L1 등록 성공 — metadata에 tx hash 포함하여 저장
							ctMetaBytes, _ := json.Marshal(map[string]interface{}{
								"url":                     "http://localhost:3004",
								"contracts":               crossTradeOutput,
								"l1_registration_tx_hash": regOutput.L2L1TxHash,
								"l1_l2l2_tx_hash":         regOutput.L2L2TxHash,
							})
							if updateErr := s.integrationRepo.UpdateMetadataAfterInstalled(
								crossTradeIntegration.ID.String(),
								entities.IntegrationInfo(ctMetaBytes),
							); updateErr != nil {
								logger.Error("failed to mark CrossTrade integration as installed",
									zap.String("stackId", stackId.String()), zap.Error(updateErr))
							}

							// Write .env.crosstrade for the CrossTrade dApp container (BE-07).
							// Non-fatal: env file failure does not block deployment success.
							envCfg := &integrations.CrossTradeDAppConfig{
								L1ChainID:              uint64(chainInformation.L1ChainID),
								L2ChainID:              uint64(chainInformation.L2ChainID),
								L2ChainName:            stackConfig.ChainName,
								L2RPCURL:               chainInformation.L2RpcUrl,
								L2BlockExplorerURL:     chainInformation.BlockExplorer,
								DeployOutput:           crossTradeOutput,
								L1CrossTradeProxyAddr:  crossTradeSepoliaL1CrossTradeProxy,
								L2toL2CrossTradeL1Addr: crossTradeSepoliaL2toL2CrossTradeL1,
							}
							if strings.EqualFold(stackConfig.FeeToken, "ETH") {
								envCfg.L2NativeTokenName = "Ethereum"
								envCfg.L2NativeTokenSymbol = "ETH"
							}
							envPath := filepath.Join(stack.DeploymentPath, "config", ".env.crosstrade")
							if envErr := integrations.BuildDAppEnvConfig(envPath, envCfg); envErr != nil {
								logger.Warn("failed to write .env.crosstrade (non-fatal)",
									zap.String("stackId", stackId.String()), zap.Error(envErr))
							}

							// BE-08: Write docker-compose.crosstrade.yml and start dApp container (non-fatal).
							// The compose file must be at stack.DeploymentPath so that its relative
							// env_file (./config/.env.crosstrade) resolves alongside the env file
							// already written above.
							crossTradeComposePath := filepath.Join(stack.DeploymentPath, "docker-compose.crosstrade.yml")
							crossTradeComposeContent := `version: "3.8"

services:
  crosstrade-dapp:
    image: tokamaknetwork/cross-trade-app:latest
    container_name: trh-crosstrade-dapp
    ports:
      - "3004:3000"
    env_file:
      - ./config/.env.crosstrade
    restart: unless-stopped
`
							if writeErr := os.WriteFile(crossTradeComposePath, []byte(crossTradeComposeContent), 0644); writeErr != nil {
								logger.Warn("failed to write docker-compose.crosstrade.yml (non-fatal)",
									zap.String("stackId", stackId.String()), zap.Error(writeErr))
							} else {
								startOut, startErr := exec.CommandContext(ctx,
									"docker", "compose", "-f", crossTradeComposePath, "up", "-d",
								).CombinedOutput()
								if startErr != nil {
									logger.Warn("failed to start CrossTrade dApp container (non-fatal)",
										zap.String("stackId", stackId.String()),
										zap.Error(startErr),
										zap.String("output", string(startOut)))
								} else {
									logger.Info("CrossTrade dApp container started",
										zap.String("stackId", stackId.String()),
										zap.String("composePath", crossTradeComposePath))
								}
							}

							// Update stack metadata with CrossTrade dApp URL.
							if stack.Metadata == nil {
								stack.Metadata = &entities.StackMetadata{}
							}
							stack.Metadata.CrossTradeUrl = "http://localhost:3004"
							if metaErr := s.stackRepo.UpdateMetadata(stackId.String(), stack.Metadata); metaErr != nil {
								logger.Warn("failed to update stack metadata with CrossTradeUrl (non-fatal)",
									zap.String("stackId", stackId.String()), zap.Error(metaErr))
							}

							logger.Info("CrossTrade auto-install completed",
								zap.String("stackId", stackId.String()),
								zap.String("l2CrossTradeProxy", crossTradeOutput.L2CrossTradeProxy),
								zap.String("l2L1TxHash", regOutput.L2L1TxHash),
								zap.String("l2L2TxHash", regOutput.L2L2TxHash),
							)
						}
					}
				}
			}
		} else {
			// AWS path: modules are deployed by SDK's installPresetModules() via Helm.
			// uptimeService uses the full service layer (creates SDK client, marks DB).
			// monitoring and DRB are already running via Helm — mark them installed in DB directly.
			if enabled, ok := def.Modules["uptimeService"]; ok && enabled {
				if _, err := s.InstallUptimeService(ctx, stackId.String()); err != nil {
					logger.Error("failed to auto-install uptime service after deployment", zap.String("stackId", stackId.String()), zap.Error(err))
				}
			}

			if enabled, ok := def.Modules["monitoring"]; ok && enabled {
				if chainInformation.MonitoringUrl == "" {
					logger.Warn("monitoring URL empty after AWS deployment; skipping installed mark",
						zap.String("stackId", stackId.String()))
				} else {
					monitoringIntegration, getErr := s.integrationRepo.GetIntegration(stackId.String(), enum.IntegrationTypeMonitoring.String())
					if getErr != nil || monitoringIntegration == nil {
						logger.Error("failed to get monitoring integration for AWS auto-mark",
							zap.String("stackId", stackId.String()), zap.Error(getErr))
					} else {
						metaBytes, metaErr := json.Marshal(map[string]string{"url": chainInformation.MonitoringUrl})
						if metaErr != nil {
							logger.Error("failed to marshal monitoring metadata for AWS",
								zap.String("stackId", stackId.String()), zap.Error(metaErr))
						} else if err := s.integrationRepo.UpdateMetadataAfterInstalled(monitoringIntegration.ID.String(), entities.IntegrationInfo(metaBytes)); err != nil {
							logger.Error("failed to mark monitoring as installed for AWS",
								zap.String("stackId", stackId.String()), zap.Error(err))
						}
					}
				}
			}

			if enabled, ok := def.Modules["drb"]; ok && enabled {
				capturedMnemonic := stackConfig.SeedPhrase
				capturedL2RPC := chainInformation.L2RpcUrl
				capturedChainID := uint64(chainInformation.L2ChainID)
				capturedStackId := stackId
				s.taskManager.AddTask(fmt.Sprintf("drb-install-%s", stackId.String()), func(ctx context.Context) {
					s.installDRBOperators(ctx, capturedStackId, capturedMnemonic, capturedL2RPC, capturedChainID)
				})
				logger.Info("DRB install task queued", zap.String("stackId", stackId.String()))
			}

			if enabled, ok := def.Modules["crossTrade"]; ok && enabled {
				pendingCT, ctErr := s.integrationRepo.GetIntegrationByStatus(
					stackId.String(),
					enum.IntegrationTypeCrossTrade.String(),
					entities.DeploymentStatusPending,
				)
				if ctErr == nil && pendingCT != nil {
					capturedConfig := stackConfig
					capturedL1ChainID := uint64(chainInformation.L1ChainID)
					capturedL2RPC := chainInformation.L2RpcUrl
					capturedL2ChainID := uint64(chainInformation.L2ChainID)
					go s.integrationMgr.AutoInstallCrossTradeAWS(
						context.Background(),
						stackId,
						&capturedConfig,
						capturedL2RPC,
						capturedL2ChainID,
						capturedL1ChainID,
					)
					logger.Info("CrossTrade auto-install goroutine launched", zap.String("stackId", stackId.String()))
				}
			}
		}

		// Start AA Operator AFTER all preset auto-installs (including CrossTrade) have
		// completed. Starting it earlier would allow the operator to consume L2 deployer
		// nonces before CrossTrade's CREATE address predictions are resolved, causing
		// waitForContractCode to poll the wrong address and time out.
		if thanosSDKConstants.NeedsAASetup(stackConfig.PresetID, stackConfig.FeeToken) {
			capturedClient := sdkClient
			s.taskManager.AddTask(fmt.Sprintf("aa-operator-%s", stackId.String()), func(ctx context.Context) {
				thanos.StartAAOperatorFromConfig(ctx, capturedClient)
			})
			logger.Info("AA Operator task queued (after CrossTrade install)",
				zap.String("stackId", stackId.String()))
		}
	}

	logger.Info("Thanos stack deployed successfully",
		zap.String("stackId", stackId.String()),
	)
}

func (s *ThanosStackDeploymentService) executeDeployments(ctx context.Context, stackId uuid.UUID) error {
	logger.Info("Updating stacks status to creating", zap.String("stackId", stackId.String()))

	err := s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusDeploying, "")
	if err != nil {
		logger.Error("failed to update stacks status",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return err
	}
	statusChan := make(chan entities.DeploymentStatusWithID)
	defer close(statusChan)

	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return fmt.Errorf("failed to get stack: %w", err)
	}

	if stack == nil {
		return fmt.Errorf("stack %s not found", stackId)
	}

	var deploymentConfig dtos.DeployThanosRequest
	if err := json.Unmarshal(stack.Config, &deploymentConfig); err != nil {
		return fmt.Errorf("failed to unmarshal stack config: %w", err)
	}

	pendingDeployments, err := s.deploymentRepo.GetDeploymentsByStackIDAndStatus(stackId.String(), entities.DeploymentRunStatusPending)
	if err != nil {
		return fmt.Errorf("failed to get deployments: %w", err)
	}

	if len(pendingDeployments) == 0 {
		return fmt.Errorf("no deployments found for stacks %s", stackId)
	}

	// Filter to only the core deployment steps we want to execute here
	filtered := make([]*entities.DeploymentEntity, 0, 2)
	var l1Step, awsStep *entities.DeploymentEntity
	for _, d := range pendingDeployments {
		if d.Step == constants.DeployL1ContractsStep {
			// keep the earliest unfinished occurrence
			if l1Step == nil || (l1Step.Status == entities.DeploymentRunStatusSuccess && d.Status != entities.DeploymentRunStatusSuccess) {
				l1Step = d
			}
		}
		if d.Step == constants.DeployAWSInfraStep || d.Step == constants.DeployLocalInfraStep {
			if awsStep == nil || (awsStep.Status == entities.DeploymentRunStatusSuccess && d.Status != entities.DeploymentRunStatusSuccess) {
				awsStep = d
			}
		}
	}
	if l1Step != nil {
		filtered = append(filtered, l1Step)
	}
	if awsStep != nil {
		filtered = append(filtered, awsStep)
	}

	// Overwrite deployments with filtered list to enforce order L1 first then AWS infra
	if len(filtered) > 0 {
		pendingDeployments = filtered
	}

	// Start a goroutine to handle status updates
	errChan := make(chan error, 1)
	go func() {
		for status := range statusChan {
			if err := s.deploymentRepo.UpdateDeploymentStatus(status.DeploymentID.String(), status.Status); err != nil {
				errChan <- fmt.Errorf("failed to update deployment status: %w", err)
				return
			}
			// If we've processed all deployments successfully, send nil to errChan
			if status.Status == entities.DeploymentRunStatusSuccess {
				select {
				case errChan <- nil:
				default:
				}
			}
		}
	}()

	for _, deployment := range pendingDeployments {
		logger.Info("Processing deployment",
			zap.String("deploymentId", deployment.ID.String()),
			zap.String("status", string(deployment.Status)),
			zap.String("step", deployment.Step))

		// Skip already completed deployments
		if deployment.Status == entities.DeploymentRunStatusSuccess {
			continue
		}

		sdkClient, err := thanos.NewThanosSDKClient(
			ctx,
			deployment.LogPath,
			toSDKNetwork(stack.Network),
			stack.DeploymentPath,
			deploymentConfig.RegisterCandidate,
			deploymentConfig.AwsAccessKey,
			deploymentConfig.AwsSecretAccessKey,
			deploymentConfig.AwsRegion,
		)
		if err != nil {
			logger.Error("failed to create thanos sdk client",
				zap.String("deploymentId", deployment.ID.String()),
				zap.Error(err))
			statusChan <- entities.DeploymentStatusWithID{
				DeploymentID: deployment.ID,
				Status:       entities.DeploymentRunStatusFailed,
			}
			return err
		}

		// Update status to in-progress before starting deployment
		statusChan <- entities.DeploymentStatusWithID{
			DeploymentID: deployment.ID,
			Status:       entities.DeploymentRunStatusInProgress,
		}

		switch deployment.Step {
		case "deploy-l1-contracts":
			var deployL1ContractsConfig dtos.DeployL1ContractsRequest
			if err := json.Unmarshal(deployment.Config, &deployL1ContractsConfig); err != nil {
				return fmt.Errorf("failed to unmarshal deployment config: %w", err)
			}

			// For local deployments, ensure Docker CLI and Compose are available before
			// running any deployment steps (Docker-in-Docker pattern requires them).
			if deploymentConfig.InfraProvider == "local" {
				if err := ensureDockerTools(); err != nil {
					statusChan <- entities.DeploymentStatusWithID{
						DeploymentID: deployment.ID,
						Status:       entities.DeploymentRunStatusFailed,
					}
					return fmt.Errorf("failed to install docker tools: %w", err)
				}
			}

			// Start log ingestion for this deployment step
			ingestCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			go s.tailAndIngestDeploymentLogs(ingestCtx, stack.ID, deployment.ID, deployment.LogPath)

			// Open deployment log file for appending tokamak-deployer output
			logFile, err := os.OpenFile(deployment.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				logger.Warn("failed to open deployment log file for binary output", zap.Error(err))
				// non-fatal: continue without file tee
			} else {
				defer logFile.Close()
				sdkClient.SetOutput(io.MultiWriter(os.Stdout, logFile))
			}

			// 60-min cap on the whole L1 deploy step (Foundry + Cannon builds +
			// tokamak-deployer). Normal runs finish in ~5 min; the cap is a final
			// safety net so a deployer-level hang (e.g., all gas-bump retries
			// exhausted) can't leave this step InProgress indefinitely.
			deployCtx, deployCancel := context.WithTimeout(ctx, 60*time.Minute)
			err = thanos.DeployL1Contracts(deployCtx, sdkClient, &deployL1ContractsConfig)
			deployCancel()
			if err != nil {
				if err == context.Canceled {
					logger.Info("deployment cancelled",
						zap.String("deploymentId", deployment.ID.String()),
						zap.String("step", deployment.Step))
					// Keep run status as-is on cancel; no explicit Stopped state in run status
					return err
				}
				if errors.Is(err, context.DeadlineExceeded) {
					logger.Error("deployment timed out (60 min cap)",
						zap.String("deploymentId", deployment.ID.String()),
						zap.String("step", deployment.Step))
				} else {
					logger.Error("deployment failed",
						zap.String("deploymentId", deployment.ID.String()),
						zap.String("step", deployment.Step),
						zap.Error(err))
				}
				statusChan <- entities.DeploymentStatusWithID{
					DeploymentID: deployment.ID,
					Status:       entities.DeploymentRunStatusFailed,
				}
				return err
			}
			statusChan <- entities.DeploymentStatusWithID{
				DeploymentID: deployment.ID,
				Status:       entities.DeploymentRunStatusSuccess,
			}
		case constants.DeployAWSInfraStep, constants.DeployLocalInfraStep:
			var deployInfraConfig dtos.DeployThanosAWSInfraRequest
			if err := json.Unmarshal(deployment.Config, &deployInfraConfig); err != nil {
				return fmt.Errorf("failed to unmarshal deployment config: %w", err)
			}

			// Start log ingestion for this deployment step
			ingestCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			go s.tailAndIngestDeploymentLogs(ingestCtx, stack.ID, deployment.ID, deployment.LogPath)

			var infraErr error
			if deployInfraConfig.InfraProvider == "local" {
				infraErr = thanos.DeployLocalInfrastructure(ctx, sdkClient, &deployInfraConfig)
				// NOTE: AA Operator is intentionally NOT started here.
				// It is started in deploy() after CrossTrade auto-install completes,
				// to prevent the AA Operator from consuming L2 deployer nonces before
				// CrossTrade contract addresses can be predicted accurately.
			} else {
				infraErr = thanos.DeployAWSInfrastructure(ctx, sdkClient, &deployInfraConfig)
			}
			if infraErr != nil {
				if errors.Is(infraErr, context.Canceled) {
					logger.Info("deployment cancelled",
						zap.String("deploymentId", deployment.ID.String()),
						zap.String("step", deployment.Step))
					return infraErr
				}
				logger.Error("deployment failed",
					zap.String("deploymentId", deployment.ID.String()),
					zap.String("step", deployment.Step),
					zap.Error(infraErr))
				statusChan <- entities.DeploymentStatusWithID{
					DeploymentID: deployment.ID,
					Status:       entities.DeploymentRunStatusFailed,
				}
				return infraErr
			}
			statusChan <- entities.DeploymentStatusWithID{
				DeploymentID: deployment.ID,
				Status:       entities.DeploymentRunStatusSuccess,
			}
		}

	}

	// Wait for final status update
	return <-errChan
}

// ---------------------------------------------------------------------------
// CrossTrade local auto-install helpers
// ---------------------------------------------------------------------------

// crossTradeSepoliaL1CrossTradeProxy is the L1CrossTradeProxy address on Sepolia.
// Deployed by 0x7220c734653ae8Ca014d4D82A84041EE4169499c (our admin key) so we can call setChainInfo.
// Implementation: 0x40dF470B38c744963CC6599C31166624B3e4d38A (reused from original)
const crossTradeSepoliaL1CrossTradeProxy = "0x5AbbFe2468F3bb34B3D5B3F72714b73aa3c1D3EB"

// crossTradeSepoliaL2toL2CrossTradeL1 is the L2toL2CrossTradeProxyL1 address on Sepolia.
// Deployed by 0x7220c734653ae8Ca014d4D82A84041EE4169499c (our admin key) so we can call setChainInfo.
// Implementation: 0x21BAF7126d2257edcCC9bfa7920b8c06A1E45e46 (reused from original)
const crossTradeSepoliaL2toL2CrossTradeL1 = "0xF09Af74810010a0e9A452f71B3921641350c21D0"

// autoInstallCrossTradeLocal deploys CrossTrade contracts on the local L2 via L1 deposit txs.
// It reads L1 contract addresses from the deployment artifacts and calls SDK DeployCrossTradeLocal.
// Called from deploy() after the main L2 stack is up (local infra, crossTrade preset enabled).
func (s *ThanosStackDeploymentService) autoInstallCrossTradeLocal(
	ctx context.Context,
	stack *entities.StackEntity,
	stackConfig *dtos.DeployThanosRequest,
	chainInfo *thanosSDKTypes.ChainInformation,
) (*thanosSDKStack.DeployCrossTradeLocalOutput, error) {
	if chainInfo.L1ChainID == 0 {
		return nil, fmt.Errorf("L1ChainID is 0: rollup.json may not be available yet")
	}

	// Read OptimismPortalProxy and L1CrossDomainMessengerProxy from deployment artifacts.
	contracts, err := trhSDKUtils.ReadDeployementConfigFromJSONFile(
		stack.DeploymentPath,
		uint64(chainInfo.L1ChainID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read deployment contracts for CrossTrade: %w", err)
	}

	if contracts.OptimismPortalProxy == "" {
		return nil, fmt.Errorf("OptimismPortalProxy address is empty in deployment artifacts")
	}
	if contracts.L1CrossDomainMessengerProxy == "" {
		return nil, fmt.Errorf("L1CrossDomainMessengerProxy address is empty in deployment artifacts")
	}

	logPath := utils.GetLogPath(stack.ID, "crosstrade-local")
	sdkClient, err := thanos.NewThanosSDKClient(
		ctx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
		false,
		stackConfig.AwsAccessKey,
		stackConfig.AwsSecretAccessKey,
		stackConfig.AwsRegion,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SDK client for CrossTrade local deploy: %w", err)
	}

	// L2 RPC URL for use inside the backend Docker container.
	// The L2 op-geth container maps port 8545 to the host; access via host gateway.
	// host.docker.internal resolves to the Docker host on macOS/Windows (Docker Desktop).
	l2RpcUrl := "http://host.docker.internal:8545"

	return thanos.DeployCrossTradeLocal(
		ctx,
		sdkClient,
		stackConfig.AdminAccount,
		stackConfig.L1RpcUrl,
		l2RpcUrl,
		uint64(chainInfo.L1ChainID),
		uint64(chainInfo.L2ChainID),
		contracts.OptimismPortalProxy,
		contracts.L1CrossDomainMessengerProxy,
		crossTradeSepoliaL1CrossTradeProxy,
		crossTradeSepoliaL2toL2CrossTradeL1,
		[]thanosSDKStack.TokenPair{
			{
				L1Token: "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238", // Sepolia USDC
				L2Token: "0x4200000000000000000000000000000000000778", // L2 USDC predeploy (Thanos)
				Symbol:  "USDC",
			},
			// TODO(usdt): Add USDT once Sepolia USDT address and L2 representation are confirmed.
		},
	)
}

// RetriggerCrossTradeLocal re-runs the CrossTrade local deployment flow on an already-deployed
// local stack whose CrossTrade integration previously failed.
// It resets the Failed integration record, re-deploys L2 contracts, registers on L1, and
// starts the dApp container — matching the auto-install flow in executeDeployments.
func (s *ThanosStackDeploymentService) RetriggerCrossTradeLocal(ctx context.Context, stackIdStr string) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackIdStr)
	if err != nil || stack == nil {
		return &entities.Response{Status: 404, Message: "stack not found"}, nil
	}
	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{Status: 400, Message: "stack is not in Deployed status"}, nil
	}

	var stackConfig dtos.DeployThanosRequest
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		return &entities.Response{Status: 500, Message: "failed to unmarshal stack config"}, err
	}
	if stackConfig.InfraProvider != "local" {
		return &entities.Response{Status: 400, Message: "RetriggerCrossTradeLocal is only for local infra stacks"}, nil
	}

	chainInfo := thanos.BuildLocalChainInformation(stack.DeploymentPath)
	if chainInfo == nil || chainInfo.L1ChainID == 0 {
		return &entities.Response{Status: 500, Message: "failed to read chain information from deployment artifacts"}, nil
	}

	// Find existing integration (may be Failed) and reset it to Pending.
	crossTradeIntegration, getErr := s.integrationRepo.GetIntegration(stackIdStr, enum.IntegrationTypeCrossTrade.String())
	if getErr != nil || crossTradeIntegration == nil {
		return &entities.Response{Status: 404, Message: "cross-trade integration record not found"}, nil
	}
	if err := s.integrationRepo.UpdateIntegrationStatus(crossTradeIntegration.ID.String(), entities.DeploymentStatusPending); err != nil {
		return &entities.Response{Status: 500, Message: "failed to reset integration status"}, err
	}

	taskId := fmt.Sprintf("retrigger-cross-trade-local-%s", stackIdStr)
	stackId := stack.ID
	s.taskManager.AddTask(taskId, func(bgCtx context.Context) {
		// 30-minute timeout for the full CrossTrade install flow.
		taskCtx, cancel := context.WithTimeout(bgCtx, 30*60*1000000000)
		defer cancel()

		if updateErr := s.integrationRepo.UpdateIntegrationStatus(crossTradeIntegration.ID.String(), entities.DeploymentStatusInProgress); updateErr != nil {
			logger.Error("retrigger: failed to set InProgress", zap.String("stackId", stackIdStr), zap.Error(updateErr))
		}

		ctOutput, ctErr := s.autoInstallCrossTradeLocal(taskCtx, stack, &stackConfig, chainInfo)
		if ctErr != nil {
			logger.Error("retrigger: CrossTrade deploy failed", zap.String("stackId", stackIdStr), zap.Error(ctErr))
			_ = s.integrationRepo.UpdateIntegrationStatusWithReason(crossTradeIntegration.ID.String(), entities.DeploymentStatusFailed, ctErr.Error())
			return
		}

		// Read L1 bridge addresses for L2toL2 setChainInfo.
		l1Contracts, contractsErr := trhSDKUtils.ReadDeployementConfigFromJSONFile(stack.DeploymentPath, uint64(chainInfo.L1ChainID))
		if contractsErr != nil {
			logger.Error("retrigger: failed to read L1 contracts for CrossTrade registration",
				zap.String("stackId", stackIdStr), zap.Error(contractsErr))
			_ = s.integrationRepo.UpdateIntegrationStatusWithReason(crossTradeIntegration.ID.String(),
				entities.DeploymentStatusFailed, fmt.Sprintf("read L1 contracts: %v", contractsErr))
			return
		}
		l1StandardBridge := l1Contracts.L1StandardBridgeProxy
		l1USDCBridge := l1Contracts.L1UsdcBridgeProxy
		l1CrossDomainMessenger := l1Contracts.L1CrossDomainMessengerProxy
		regInput := &integrations.CrossTradeL1RegistrationInput{
			L1RPCURL:               stackConfig.L1RpcUrl,
			L1ChainID:              uint64(chainInfo.L1ChainID),
			L2ChainID:              uint64(chainInfo.L2ChainID),
			DeployerPrivKey:        stackConfig.AdminAccount,
			L2CrossTradeProxy:      ctOutput.L2CrossTradeProxy,
			L2toL2CrossTradeProxy:  ctOutput.L2toL2CrossTradeProxy,
			L1StandardBridge:       l1StandardBridge,
			L1USDCBridge:           l1USDCBridge,
			L1CrossDomainMessenger: l1CrossDomainMessenger,
		}
		regOutput, regErr := integrations.RegisterCrossTradeL2(taskCtx, regInput, 3)
		if regErr != nil {
			logger.Error("retrigger: L1 registration failed", zap.String("stackId", stackIdStr), zap.Error(regErr))
			_ = s.integrationRepo.UpdateIntegrationStatusWithReason(crossTradeIntegration.ID.String(), entities.DeploymentStatusFailed, regErr.Error())
			return
		}

		ctMetaBytes, _ := json.Marshal(map[string]interface{}{
			"url":                     "http://localhost:3004",
			"contracts":               ctOutput,
			"l1_registration_tx_hash": regOutput.L2L1TxHash,
			"l1_l2l2_tx_hash":         regOutput.L2L2TxHash,
		})
		if updateErr := s.integrationRepo.UpdateMetadataAfterInstalled(crossTradeIntegration.ID.String(), entities.IntegrationInfo(ctMetaBytes)); updateErr != nil {
			logger.Error("retrigger: failed to mark as installed", zap.String("stackId", stackIdStr), zap.Error(updateErr))
			return
		}

		// Write .env.crosstrade and start dApp container.
		envCfg := &integrations.CrossTradeDAppConfig{
			L1ChainID:              uint64(chainInfo.L1ChainID),
			L2ChainID:              uint64(chainInfo.L2ChainID),
			L2ChainName:            stackConfig.ChainName,
			L2RPCURL:               chainInfo.L2RpcUrl,
			L2BlockExplorerURL:     chainInfo.BlockExplorer,
			DeployOutput:           ctOutput,
			L1CrossTradeProxyAddr:  crossTradeSepoliaL1CrossTradeProxy,
			L2toL2CrossTradeL1Addr: crossTradeSepoliaL2toL2CrossTradeL1,
		}
		if strings.EqualFold(stackConfig.FeeToken, "ETH") {
			envCfg.L2NativeTokenName = "Ethereum"
			envCfg.L2NativeTokenSymbol = "ETH"
		}
		envPath := filepath.Join(stack.DeploymentPath, "config", ".env.crosstrade")
		if envErr := integrations.BuildDAppEnvConfig(envPath, envCfg); envErr != nil {
			logger.Warn("retrigger: failed to write .env.crosstrade (non-fatal)", zap.String("stackId", stackIdStr), zap.Error(envErr))
		}

		crossTradeComposePath := filepath.Join(stack.DeploymentPath, "docker-compose.crosstrade.yml")
		crossTradeComposeContent := `version: "3.8"

services:
  crosstrade-dapp:
    image: tokamaknetwork/cross-trade-app:latest
    container_name: trh-crosstrade-dapp
    ports:
      - "3004:3000"
    env_file:
      - ./config/.env.crosstrade
    restart: unless-stopped
`
		if writeErr := os.WriteFile(crossTradeComposePath, []byte(crossTradeComposeContent), 0644); writeErr != nil {
			logger.Warn("retrigger: failed to write compose file (non-fatal)", zap.String("stackId", stackIdStr), zap.Error(writeErr))
		} else {
			startOut, startErr := exec.CommandContext(taskCtx, "docker", "compose", "-f", crossTradeComposePath, "up", "-d").CombinedOutput()
			if startErr != nil {
				logger.Warn("retrigger: failed to start dApp container (non-fatal)", zap.String("stackId", stackIdStr), zap.Error(startErr), zap.String("output", string(startOut)))
			} else {
				logger.Info("retrigger: CrossTrade dApp container started", zap.String("stackId", stackIdStr))
			}
		}

		if stack.Metadata == nil {
			stack.Metadata = &entities.StackMetadata{}
		}
		stack.Metadata.CrossTradeUrl = "http://localhost:3004"
		if metaErr := s.stackRepo.UpdateMetadata(stackId.String(), stack.Metadata); metaErr != nil {
			logger.Warn("retrigger: failed to update stack metadata (non-fatal)", zap.String("stackId", stackIdStr), zap.Error(metaErr))
		}

		logger.Info("retrigger: CrossTrade install completed",
			zap.String("stackId", stackIdStr),
			zap.String("l2CrossTradeProxy", ctOutput.L2CrossTradeProxy),
		)
	})

	return &entities.Response{Status: 200, Message: "CrossTrade local re-deployment started"}, nil
}

// installDRBOperators deploys DRB Leader (EKS) and Regular nodes (EC2) after L2 is running.
// Runs as a background goroutine. Follows the AA operator pattern.
func (s *ThanosStackDeploymentService) installDRBOperators(ctx context.Context, stackId uuid.UUID, mnemonic string, l2RPCURL string, chainID uint64) {
	logger := zap.L().With(zap.String("stackId", stackId.String()))

	drbIntegration, err := s.integrationRepo.GetIntegration(stackId.String(), enum.IntegrationTypeDRB.String())
	if err != nil || drbIntegration == nil {
		logger.Error("DRB integration not found", zap.Error(err))
		return
	}

	markFailed := func(reason string) {
		if updateErr := s.integrationRepo.UpdateIntegrationStatusWithReason(
			drbIntegration.ID.String(),
			entities.DeploymentStatusFailed,
			reason,
		); updateErr != nil {
			logger.Error("failed to mark DRB as failed", zap.Error(updateErr))
		}
	}

	// Step 1: Derive accounts
	leaderKey, regularKeys, err := DeriveDRBAccounts(mnemonic)
	if err != nil {
		logger.Error("DRB account derivation failed", zap.Error(err))
		markFailed(fmt.Sprintf("account derivation failed: %v", err))
		return
	}

	// Step 2: Fund regular accounts from admin (admin key = leader key = BIP44 idx 0)
	if err := fundDRBRegularAccounts(ctx, l2RPCURL, leaderKey, regularKeys); err != nil {
		logger.Error("DRB regular funding failed", zap.Error(err))
		markFailed(fmt.Sprintf("regular funding failed: %v", err))
		return
	}

	// Step 3: Get stack config to create SDK client
	stack, sdkErr := s.stackRepo.GetStackByID(stackId.String())
	if sdkErr != nil {
		logger.Error("failed to get stack for DRB deploy", zap.Error(sdkErr))
		markFailed(fmt.Sprintf("stack not found: %v", sdkErr))
		return
	}

	config, marshalErr := json.Marshal(stack.Config)
	if marshalErr != nil {
		logger.Error("failed to marshal stack config", zap.Error(marshalErr))
		markFailed(fmt.Sprintf("config marshal error: %v", marshalErr))
		return
	}

	var stackConfig dtos.DeployThanosRequest
	if unmarshalErr := json.Unmarshal(config, &stackConfig); unmarshalErr != nil {
		logger.Error("failed to unmarshal stack config", zap.Error(unmarshalErr))
		markFailed(fmt.Sprintf("config unmarshal error: %v", unmarshalErr))
		return
	}

	logPath := utils.GetLogPath(stackId, "drb-install")
	sdkClient, sdkErr := thanos.NewThanosSDKClient(
		ctx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
		stackConfig.RegisterCandidate,
		stackConfig.AwsAccessKey,
		stackConfig.AwsSecretAccessKey,
		stackConfig.AwsRegion,
	)
	if sdkErr != nil {
		logger.Error("failed to create SDK client for DRB deploy", zap.Error(sdkErr))
		markFailed(fmt.Sprintf("SDK client error: %v", sdkErr))
		return
	}

	deployConfig := sdkClient.GetDeployConfig()
	existingCluster := ""
	region := ""
	if deployConfig != nil {
		if deployConfig.K8s != nil {
			existingCluster = deployConfig.K8s.Namespace
		}
		if deployConfig.AWS != nil {
			region = deployConfig.AWS.Region
		}
	}

	// Step 4: Deploy DRB Leader (EKS) + Regular nodes (EC2) via SDK
	drbOutput, err := thanos.DeployDRBFromConfig(ctx, sdkClient, &thanosSDKTypes.DeployDRBInput{
		RPC:                     l2RPCURL,
		ChainID:                 chainID,
		PrivateKey:              leaderKey,
		ExistingContractAddress: "0x4200000000000000000000000000000000000060",
		ExistingClusterName:     existingCluster,
		DatabaseConfig:          &thanosSDKTypes.DRBDatabaseConfig{Type: "local"},
	})
	if err != nil {
		logger.Error("DRB deployment failed", zap.Error(err))
		markFailed(fmt.Sprintf("DRB deploy failed: %v", err))
		return
	}

	// Step 5: Activate regular nodes (depositAndActivate)
	activateErr := thanos.ActivateRegularOperatorsFromConfig(ctx, sdkClient, l2RPCURL,
		"0x4200000000000000000000000000000000000060", regularKeys)
	if activateErr != nil {
		logger.Error("DRB regular activation failed (partially installed)", zap.Error(activateErr))
		if updateErr := s.integrationRepo.UpdateIntegrationStatusWithReason(
			drbIntegration.ID.String(),
			entities.DeploymentStatusFailed,
			activateErr.Error(),
		); updateErr != nil {
			logger.Error("failed to mark DRB as failed after activation error", zap.Error(updateErr))
		}
		return
	}

	// Step 6: Mark as installed with leader URL
	leaderURL := ""
	if drbOutput != nil && drbOutput.DeployDRBApplicationOutput != nil {
		leaderURL = drbOutput.DeployDRBApplicationOutput.LeaderNodeURL
	}
	metaBytes, err := json.Marshal(map[string]string{"url": leaderURL, "cluster": existingCluster, "region": region})
	if err != nil {
		logger.Error("failed to marshal DRB metadata", zap.Error(err))
		return
	}
	if err := s.integrationRepo.UpdateMetadataAfterInstalled(drbIntegration.ID.String(), entities.IntegrationInfo(metaBytes)); err != nil {
		logger.Error("failed to mark DRB as installed", zap.Error(err))
		return
	}
	logger.Info("DRB operators installed and activated", zap.String("leaderURL", leaderURL))
}

// fundDRBRegularAccounts sends 3 TON from admin to each regular account for DRB activation.
func fundDRBRegularAccounts(ctx context.Context, l2RPCURL string, adminPrivKeyHex string, regularKeys []string) error {
	client, err := ethclient.DialContext(ctx, l2RPCURL)
	if err != nil {
		return fmt.Errorf("failed to connect to L2 RPC: %w", err)
	}
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get chain ID: %w", err)
	}

	adminKey, err := ethCrypto.HexToECDSA(adminPrivKeyHex)
	if err != nil {
		return fmt.Errorf("invalid admin key: %w", err)
	}
	adminAddr := ethCrypto.PubkeyToAddress(adminKey.PublicKey)

	// 3 TON = 3e18 wei
	amount := new(big.Int).Mul(big.NewInt(3), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	signer := types.LatestSignerForChainID(chainID)

	for i, privKeyHex := range regularKeys {
		regularKey, err := ethCrypto.HexToECDSA(privKeyHex)
		if err != nil {
			return fmt.Errorf("regular[%d]: invalid key: %w", i, err)
		}
		toAddr := ethCrypto.PubkeyToAddress(regularKey.PublicKey)

		nonce, err := client.PendingNonceAt(ctx, adminAddr)
		if err != nil {
			return fmt.Errorf("regular[%d]: failed to get nonce: %w", i, err)
		}
		gasPrice, err := client.SuggestGasPrice(ctx)
		if err != nil {
			return fmt.Errorf("regular[%d]: failed to get gas price: %w", i, err)
		}

		tx := types.NewTransaction(nonce, toAddr, amount, 21000, gasPrice, nil)
		signed, err := types.SignTx(tx, signer, adminKey)
		if err != nil {
			return fmt.Errorf("regular[%d]: sign error: %w", i, err)
		}
		if err := client.SendTransaction(ctx, signed); err != nil {
			return fmt.Errorf("regular[%d]: send error: %w", i, err)
		}

		for {
			receipt, err := client.TransactionReceipt(ctx, signed.Hash())
			if err == nil {
				if receipt.Status == 0 {
					return fmt.Errorf("regular[%d]: funding tx reverted", i)
				}
				break
			}
			if err != ethereum.NotFound {
				return fmt.Errorf("regular[%d]: receipt error: %w", i, err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
	return nil
}
