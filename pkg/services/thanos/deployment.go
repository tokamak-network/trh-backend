package thanos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/constants"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/enum"
	"github.com/tokamak-network/trh-backend/pkg/services/thanos/presets"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	"go.uber.org/zap"
)

// New helper method to handle deployment logic
func (s *ThanosStackDeploymentService) deploy(ctx context.Context, stackId uuid.UUID) {
	err := s.executeDeployments(ctx, stackId)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			logger.Info("deployment cancelled", zap.String("stackId", stackId.String()))
			// Set status to Stopped so the user can Resume later
			if updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusStopped, "deployment cancelled"); updateErr != nil {
				logger.Error("failed to update stack status after cancellation",
					zap.String("stackId", stackId.String()),
					zap.Error(updateErr))
			}
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

	// Local target: populate metadata from stack config directly (no AWS SDK client needed)
	if stack.ResolveTarget() == entities.DeploymentTargetLocal {
		var localConfig dtos.DeployThanosRequest
		if cfgBytes, marshalErr := json.Marshal(stack.Config); marshalErr == nil {
			if json.Unmarshal(cfgBytes, &localConfig) == nil {
				namespace := fmt.Sprintf("locall2-%s", stackId.String()[:5])
				l2RpcUrl := fmt.Sprintf("http://localhost:8545") // port-forward target
				l2ChainIDVal := 0
				// Try to read l2ChainID from deploy config if available
				deployConfigPath := fmt.Sprintf("%s/tokamak-thanos/packages/tokamak/contracts-bedrock/deploy-config/tmp/config.json", stack.DeploymentPath)
				if raw, readErr := os.ReadFile(deployConfigPath); readErr == nil {
					var deployCfg map[string]interface{}
					if json.Unmarshal(raw, &deployCfg) == nil {
						if v, ok := deployCfg["l2ChainID"]; ok {
							if fv, ok := v.(float64); ok {
								l2ChainIDVal = int(fv)
							}
						}
					}
				}

				metaErr := s.stackRepo.UpdateMetadata(stackId.String(), &entities.StackMetadata{
					Layer1:    "Ethereum Sepolia",
					Layer2:    "Thanos Stack (Local)",
					L1ChainId: 11155111,
					L2ChainId: l2ChainIDVal,
					L2RpcUrl:  l2RpcUrl,
				})
				if metaErr != nil {
					logger.Error("failed to update local stack metadata", zap.Error(metaErr))
				} else {
					logger.Info("LocalTestnet metadata updated",
						zap.String("stackId", stackId.String()),
						zap.String("namespace", namespace),
						zap.Int("l2ChainId", l2ChainIDVal))
				}
			}
		}
		return
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

	// Get chain information
	chainInformation, err := thanos.ShowChainInformation(ctx, sdkClient)
	if err != nil || chainInformation == nil {
		logger.Error("failed to show chain information", zap.Error(err))
		return
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
	if bridgeUrl == "" {
		logger.Error("bridge url is empty", zap.String("stackId", stackId.String()))
		return
	}

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

	if stackConfig.RegisterCandidate {
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

	// Auto-install preset modules that don't require user configuration
	if stackConfig.PresetID != "" {
		presetSvc := presets.NewService()
		def, err := presetSvc.GetByID(stackConfig.PresetID)
		if err != nil {
			logger.Error("failed to get preset definition for auto-install", zap.String("presetId", stackConfig.PresetID), zap.Error(err))
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
	filtered := make([]*entities.DeploymentEntity, 0, 3)
	var buildStep, l1Step, awsStep, localInfraStep *entities.DeploymentEntity
	for _, d := range pendingDeployments {
		if d.Step == constants.BuildL1ContractsStep {
			if buildStep == nil || (buildStep.Status == entities.DeploymentRunStatusSuccess && d.Status != entities.DeploymentRunStatusSuccess) {
				buildStep = d
			}
		}
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
		if d.Step == constants.DeployLocalInfraStep {
			if localInfraStep == nil || (localInfraStep.Status == entities.DeploymentRunStatusSuccess && d.Status != entities.DeploymentRunStatusSuccess) {
				localInfraStep = d
			}
		}
	}
	// Order: build → deploy-l1-contracts → infra
	if buildStep != nil {
		filtered = append(filtered, buildStep)
	}
	if l1Step != nil {
		filtered = append(filtered, l1Step)
	}
	if awsStep != nil {
		filtered = append(filtered, awsStep)
	}
	if localInfraStep != nil {
		filtered = append(filtered, localInfraStep)
	}

	// Overwrite deployments with filtered list to enforce order L1 first then infra
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
		case constants.BuildL1ContractsStep:
			var buildConfig dtos.DeployL1ContractsRequest
			if err := json.Unmarshal(deployment.Config, &buildConfig); err != nil {
				return fmt.Errorf("failed to unmarshal build config: %w", err)
			}

			// Start log ingestion for this deployment step
			ingestCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			go s.tailAndIngestDeploymentLogs(ingestCtx, stack.ID, deployment.ID, deployment.LogPath)

			if err := thanos.DeployL1Contracts(ctx, sdkClient, &buildConfig); err != nil {
				if errors.Is(err, context.Canceled) {
					logger.Info("build cancelled",
						zap.String("deploymentId", deployment.ID.String()),
						zap.String("step", deployment.Step))
					return err
				}
				logger.Error("build failed",
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

		case "deploy-l1-contracts":
			var deployL1ContractsConfig dtos.DeployL1ContractsRequest
			if err := json.Unmarshal(deployment.Config, &deployL1ContractsConfig); err != nil {
				return fmt.Errorf("failed to unmarshal deployment config: %w", err)
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
