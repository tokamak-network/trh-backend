package thanos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

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
						ctMetaBytes, _ := json.Marshal(map[string]interface{}{
							"url":       "http://localhost:3004",
							"contracts": crossTradeOutput,
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
						envPath := filepath.Join(stack.DeploymentPath, "config", ".env.crosstrade")
						if envErr := integrations.BuildDAppEnvConfig(envPath, envCfg); envErr != nil {
							logger.Warn("failed to write .env.crosstrade (non-fatal)",
								zap.String("stackId", stackId.String()), zap.Error(envErr))
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
						)
					}
				}
			}
		} else {
			if enabled, ok := def.Modules["uptimeService"]; ok && enabled {
				if _, err := s.InstallUptimeService(ctx, stackId.String()); err != nil {
					logger.Error("failed to auto-install uptime service after deployment", zap.String("stackId", stackId.String()), zap.Error(err))
				}
			}
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
		if d.Step == constants.DeployInfraStep {
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
			strings.ToLower(string(stack.Network)),
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

			if err := thanos.DeployL1Contracts(ctx, sdkClient, &deployL1ContractsConfig); err != nil {
				if err == context.Canceled {
					logger.Info("deployment cancelled",
						zap.String("deploymentId", deployment.ID.String()),
						zap.String("step", deployment.Step))
					// Keep run status as-is on cancel; no explicit Stopped state in run status
					return err
				}
				logger.Error("deployment failed",
					zap.String("deploymentId", deployment.ID.String()),
					zap.String("step", deployment.Step),
					zap.Error(err))
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
		case "deploy-aws-infra":
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
				if infraErr == nil && thanosSDKConstants.NeedsAASetup(deploymentConfig.PresetID, deploymentConfig.FeeToken) {
					capturedClient := sdkClient
					s.taskManager.AddTask(fmt.Sprintf("aa-operator-%s", stackId.String()), func(ctx context.Context) {
						thanos.StartAAOperatorFromConfig(ctx, capturedClient)
					})
				}
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

// crossTradeSepoliaL1CrossTradeProxy is the pre-deployed L1CrossTradeProxy address on Sepolia.
// Used for local L2 deployments: setChainInfo is called against this L1 proxy via deposit tx.
const crossTradeSepoliaL1CrossTradeProxy = "0xf3473E20F1d9EB4468C72454a27aA1C65B67AB35"

// crossTradeSepoliaL2toL2CrossTradeL1 is the pre-deployed L2toL2CrossTradeL1 address on Sepolia.
// Used for local L2 deployments: L2toL2 setChainInfo points at this L1 contract.
const crossTradeSepoliaL2toL2CrossTradeL1 = "0xDa2CbF69352cB46d9816dF934402b421d93b6BC2"

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

	return thanos.DeployCrossTradeLocal(
		ctx,
		sdkClient,
		stackConfig.AdminAccount,
		stackConfig.L1RpcUrl,
		uint64(chainInfo.L1ChainID),
		uint64(chainInfo.L2ChainID),
		contracts.OptimismPortalProxy,
		contracts.L1CrossDomainMessengerProxy,
		crossTradeSepoliaL1CrossTradeProxy,
		crossTradeSepoliaL2toL2CrossTradeL1,
		[]thanosSDKStack.TokenPair{},
	)
}
