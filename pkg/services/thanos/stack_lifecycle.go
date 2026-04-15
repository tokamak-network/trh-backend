package thanos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/enum"
	"github.com/tokamak-network/trh-backend/pkg/services/thanos/presets"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	"go.uber.org/zap"
)

// activeLocalStatuses are stack statuses that indicate a local stack is
// occupying fixed ports and must not be overlapped by a new local deployment.
var activeLocalStatuses = map[entities.StackStatus]bool{
	entities.StackStatusPending:   true,
	entities.StackStatusDeploying: true,
	entities.StackStatusDeployed:  true,
	entities.StackStatusUpdating:  true,
}

// checkNoActiveLocalStack returns a 409 response if any existing local stack is
// in an active state. Local deployments use fixed ports, so concurrent deploys
// would cause port conflicts.
//
// Before blocking, it reconciles DB state with Docker reality: if a stack is
// marked active in DB but has no running containers, it was likely removed
// manually outside the app. In that case the DB record is auto-corrected to
// Terminated so a new deployment is not blocked.
func (s *ThanosStackDeploymentService) checkNoActiveLocalStack() *entities.Response {
	stacks, err := s.stackRepo.GetAllStacks()
	if err != nil {
		// If we can't query, fail open — don't block the deploy on a DB read error.
		logger.Warn("checkNoActiveLocalStack: failed to query stacks, skipping check", zap.Error(err))
		return nil
	}
	for _, st := range stacks {
		if !activeLocalStatuses[st.Status] {
			continue
		}
		var cfg struct {
			InfraProvider string `json:"infraProvider"`
		}
		if err := json.Unmarshal(st.Config, &cfg); err != nil {
			continue
		}
		if cfg.InfraProvider != "local" {
			continue
		}

		// Reconcile: check whether Docker containers are actually running.
		// If the stack was manually torn down outside the app, DB state is stale.
		projectName := filepath.Base(st.DeploymentPath)
		if projectName != "" && projectName != "." {
			running, checkErr := hasRunningContainersForProject(projectName)
			if checkErr != nil {
				logger.Warn("checkNoActiveLocalStack: docker check failed, treating stack as active",
					zap.String("stackId", st.ID.String()),
					zap.Error(checkErr))
			} else if !running {
				logger.Warn("checkNoActiveLocalStack: DB shows active but no containers running; auto-correcting to Terminated",
					zap.String("stackId", st.ID.String()),
					zap.String("status", string(st.Status)))
				if updateErr := s.stackRepo.UpdateStatus(st.ID.String(), entities.StackStatusTerminated, "auto-terminated: no containers found"); updateErr != nil {
					logger.Error("checkNoActiveLocalStack: failed to auto-correct stale stack status", zap.Error(updateErr))
				}
				continue
			}
		}

		return &entities.Response{
			Status:  http.StatusConflict,
			Message: fmt.Sprintf("a local stack is already active (stackId: %s, status: %s): stop it before starting a new local deployment", st.ID, st.Status),
		}
	}
	return nil
}

// hasRunningContainersForProject returns true if any Docker container belonging
// to the given compose project is currently running.
func hasRunningContainersForProject(projectName string) (bool, error) {
	out, err := exec.Command("docker", "ps",
		"--filter", "label=com.docker.compose.project="+projectName,
		"-q").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func (s *ThanosStackDeploymentService) CreateThanosStack(
	ctx context.Context,
	request dtos.DeployThanosRequest,
) (*entities.Response, error) {
	// Guard: local deployments use fixed ports — block if another local stack is active.
	if request.InfraProvider == "local" {
		if conflict := s.checkNoActiveLocalStack(); conflict != nil {
			return conflict, nil
		}
	}

	stackId := uuid.New()
	deploymentPath := utils.GetDeploymentPath(s.name, request.Network, stackId.String())
	request.DeploymentPath = deploymentPath
	config, err := json.Marshal(request)
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}
	var mainnetConfirmation json.RawMessage
	var immutableConfig json.RawMessage

	if request.Network == entities.DeploymentNetworkMainnet {
		if request.MainnetConfirmation == nil {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "Mainnet confirmation is required for mainnet deployments",
				Data:    nil,
			}, nil
		}
		bytes, err := json.Marshal(request.MainnetConfirmation)
		if err != nil {
			return nil, err
		}
		mainnetConfirmation = bytes

		immConfig := map[string]interface{}{
			"l2BlockTime":              request.L2BlockTime,
			"challengePeriod":          request.ChallengePeriod,
			"batchSubmissionFrequency": request.BatchSubmissionFrequency,
			"outputRootFrequency":      request.OutputRootFrequency,
			"chainName":                request.ChainName,
			"adminAccount":             request.AdminAccount,
			"sequencerAccount":         request.SequencerAccount,
			"batcherAccount":           request.BatcherAccount,
			"proposerAccount":          request.ProposerAccount,
			"challengerAccount":        request.ChallengerAccount,
			"enableFaultProof":         request.EnableFaultProof,
		}
		bytes, err = json.Marshal(immConfig)
		if err != nil {
			return nil, err
		}
		immutableConfig = bytes
	}

	stack := &entities.StackEntity{
		ID:                  stackId,
		Name:                s.name,
		Network:             request.Network,
		Type:                enum.StackTypeOptimisticRollup.String(),
		Config:              config,
		DeploymentPath:      deploymentPath,
		Status:              entities.StackStatusPending,
		ImmutableConfig:     immutableConfig,
		MainnetConfirmation: mainnetConfirmation,
	}

	// We install the bridge by default
	integrations := make([]*entities.IntegrationEntity, 0)
	bridgeIntegration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    enum.IntegrationTypeBridge.String(),
		Status:  string(entities.DeploymentStatusPending),
	}
	integrations = append(integrations, bridgeIntegration)

	if request.RegisterCandidate {
		registerCandidateIntegration := &entities.IntegrationEntity{
			ID:      uuid.New(),
			StackID: &stack.ID,
			Type:    enum.IntegrationTypeRegisterCandidate.String(),
			Status:  string(entities.DeploymentStatusPending),
		}
		integrations = append(integrations, registerCandidateIntegration)
	}

	if request.PresetID != "" {
		presetSvc := presets.NewService()
		def, err := presetSvc.GetByID(request.PresetID)
		if err != nil {
			logger.Error("unknown preset id, skipping preset module integration creation", zap.String("presetId", request.PresetID), zap.Error(err))
		} else {
			moduleToType := map[string]enum.IntegrationType{
				"uptimeService": enum.IntegrationTypeUptimeService,
				"crossTrade":    enum.IntegrationTypeCrossTrade,
				"blockExplorer": enum.IntegrationTypeBlockExplorer,
				"monitoring":    enum.IntegrationTypeMonitoring,
				"drb":           enum.IntegrationTypeDRB,
			}
			// Modules that require user configuration before installation
			deferredModules := map[string]bool{
				"blockExplorer": true,
				"monitoring":    true,
			}
			for module, enabled := range def.Modules {
				if !enabled || module == "bridge" {
					continue // bridge is always created above
				}
				intType, ok := moduleToType[module]
				if !ok {
					continue
				}
				status := string(entities.DeploymentStatusPending)
				if deferredModules[module] {
					status = string(entities.DeploymentStatusAwaitingConfig)
				}
				integrations = append(integrations, &entities.IntegrationEntity{
					ID:      uuid.New(),
					StackID: &stack.ID,
					Type:    intType.String(),
					Status:  status,
				})
			}
			// Apply preset registerCandidate default if not already set
			if rc, ok := def.ChainDefaults["registerCandidate"].(bool); ok && rc && !request.RegisterCandidate {
				request.RegisterCandidate = true
				integrations = append(integrations, &entities.IntegrationEntity{
					ID:      uuid.New(),
					StackID: &stack.ID,
					Type:    enum.IntegrationTypeRegisterCandidate.String(),
					Status:  string(entities.DeploymentStatusPending),
				})
			}
		}
	}

	deployments, err := s.getThanosStackDeployments(stackId, &request)
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	err = s.stackRepo.CreateStackByTx(stack, deployments, integrations)
	if err != nil {
		logger.Error("Failed to create thanos stack", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	logger.Info("Stack created", zap.String("stackId", stackId.String()))

	taskId := fmt.Sprintf("deploy-thanos-stack-%s", stackId.String())
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		s.deploy(ctx, stackId)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]string{"stackId": stackId.String()},
	}, nil
}

func (s *ThanosStackDeploymentService) StopDeployingThanosStack(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Stack not found",
			Data:    nil,
		}, nil
	}

	if stack.Status != entities.StackStatusDeploying {
		if stack.Status == entities.StackStatusDeployed {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "Stack is already deployed, if you want to stop it, please use the terminate it",
				Data:    nil,
			}, nil
		}
		if stack.Status == entities.StackStatusTerminating {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "Stack is terminating, please wait for it to finish",
				Data:    nil,
			}, nil
		}

		if stack.Status == entities.StackStatusTerminated {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "Stack is terminated, you cannot stop it",
				Data:    nil,
			}, nil
		}

		if stack.Status == entities.StackStatusFailedToDeploy {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "Stack failed to deploy, you cannot stop it",
				Data:    nil,
			}, nil
		}

		if stack.Status == entities.StackStatusFailedToTerminate {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "Stack failed to terminate, you cannot stop it",
				Data:    nil,
			}, nil
		}

		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deploying, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	taskId := fmt.Sprintf("deploy-thanos-stack-%s", stackId.String())
	s.taskManager.StopTask(taskId)
	// Update stacks status to stopping
	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusStopped, "")
	if err != nil {
		logger.Error("failed to update stacks status",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}
	err = s.deploymentRepo.UpdateStatusesByStackId(stackId.String(), entities.DeploymentRunStatusStopped)
	if err != nil {
		logger.Error("failed to update deployment status",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
	}

	err = s.integrationRepo.UpdateIntegrationStatusByStackID(stackId.String(), entities.DeploymentStatusStopped)
	if err != nil {
		logger.Error("failed to update integration status",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) ResumeThanosStack(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Stack not found",
			Data:    nil,
		}, nil
	}

	if stack.Status != entities.StackStatusStopped &&
		stack.Status != entities.StackStatusFailedToDeploy && stack.Status != entities.StackStatusTerminated {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not stopped, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}
	// Create fresh deployment records for this resume
	var stackConfig dtos.DeployThanosRequest
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	// Get stopped pendingDeployments
	pendingDeployments, err := s.getThanosStackDeployments(stackId, &stackConfig)
	if err != nil {
		logger.Error("failed to get deployments", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	for _, d := range pendingDeployments {
		err = s.deploymentRepo.CreateDeployment(d)
		if err != nil {
			logger.Error("failed to create deployment record on resume", zap.String("stackId", stackId.String()), zap.Error(err))
			return &entities.Response{
				Status:  http.StatusInternalServerError,
				Message: "Internal server error",
				Data:    nil,
			}, err
		}
	}

	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusPending, "")
	if err != nil {
		logger.Error("failed to update stack status to pending", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("deploy-thanos-stack-%s", stackId.String())
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		s.deploy(ctx, stackId)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) UpdateNetwork(ctx context.Context, stackId uuid.UUID, request dtos.UpdateNetworkRequest) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Stack not found",
			Data:    nil,
		}, nil
	}

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	logPath := utils.GetLogPath(stack.ID, "update-network")
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
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusUpdating, "")
	if err != nil {
		logger.Error("failed to update stack status", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("update-network-%s", stackId.String())
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		err = thanos.UpdateNetwork(ctx, sdkClient, &request)
		if err != nil {
			logger.Error("failed to update network", zap.Error(err))
			return
		}

		// Update stack config with new L1RPC and L1Beacon URLs
		stackConfig.L1RpcUrl = request.L1RpcUrl
		stackConfig.L1BeaconUrl = request.L1BeaconUrl

		updatedConfig, err := json.Marshal(stackConfig)
		if err != nil {
			logger.Error("failed to marshal updated stack config", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}

		err = s.stackRepo.UpdateConfig(stackId.String(), updatedConfig)
		if err != nil {
			logger.Error("failed to update stack config", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}

		err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusDeployed, "")
		if err != nil {
			logger.Error("failed to update stack status", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) TerminateThanosStack(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	// Check if stacks exists
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	// Check if stacks is in a valid state to be terminated
	if stack.Status == entities.StackStatusDeploying || stack.Status == entities.StackStatusUpdating ||
		stack.Status == entities.StackStatusTerminating {
		logger.Error(
			"The stacks is still deploying, updating or terminating, please wait for it to finish",
			zap.String("stackId", stackId.String()),
		)
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "The stacks is still deploying, updating or terminating, please wait for it to finish",
			Data:    nil,
		}, nil
	}

	// Update stack status to pending termination
	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusPending, "")
	if err != nil {
		logger.Error("failed to update stack status to pending termination",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to update stack status to pending termination",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("terminate-thanos-stack-%s", stackId.String())
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		s.handleStackTermination(ctx, stack)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}
