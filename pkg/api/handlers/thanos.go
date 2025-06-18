package handlers

import (
	"net/http"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/api/servers"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	postgresRepositories "github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/repositories"
	"github.com/tokamak-network/trh-backend/pkg/services"
	"github.com/tokamak-network/trh-backend/pkg/taskmanager"
)

type ThanosDeploymentHandler struct {
	ThanosDeploymentService *services.ThanosStackDeploymentService
}

func (h *ThanosDeploymentHandler) Deploy(c *gin.Context) {
	var request dtos.DeployThanosRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	if err := request.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	request.AdminAccount = utils.TrimPrivateKey(request.AdminAccount)
	request.SequencerAccount = utils.TrimPrivateKey(request.SequencerAccount)
	request.BatcherAccount = utils.TrimPrivateKey(request.BatcherAccount)
	request.ProposerAccount = utils.TrimPrivateKey(request.ProposerAccount)

	response, err := h.ThanosDeploymentService.CreateThanosStack(c, request)
	if err != nil {
		logger.Error("failed to deploy thanos stack", zap.Error(err))
	}

	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) Stop(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.StopDeployingThanosStack(c, uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to stop thanos stack", zap.Error(err))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) UpdateNetwork(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	var request dtos.UpdateNetworkRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.UpdateNetwork(c, uuid.MustParse(id), request)
	if err != nil {
		logger.Error("failed to update network", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) Terminate(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.TerminateThanosStack(c, uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to terminate thanos stack", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) Resume(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.ResumeThanosStack(c, uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to resume thanos stack", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) GetAllStacks(c *gin.Context) {
	response, err := h.ThanosDeploymentService.GetAllStacks()
	if err != nil {
		logger.Error("failed to get all stacks", zap.Error(err))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) GetStackStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.GetStackStatus(uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to get stack status", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) GetDeployments(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.GetDeployments(uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to get deployments", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) GetIntegrations(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.GetIntegrations(uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to get integrations", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) GetIntegrationById(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	integrationId := c.Param("integrationId")
	if integrationId == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "integrationId is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.GetIntegration(
		uuid.MustParse(id),
		uuid.MustParse(integrationId),
	)
	if err != nil {
		logger.Error("failed to get integration", zap.Error(err), zap.String("id", id), zap.String("integrationId", integrationId))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) GetStackDeployment(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	deploymentId := c.Param("deploymentId")
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "deploymentId is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.GetStackDeployment(
		uuid.MustParse(id),
		uuid.MustParse(deploymentId),
	)
	if err != nil {
		logger.Error("failed to get stack deployment", zap.Error(err), zap.String("id", id), zap.String("deploymentId", deploymentId))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) GetStackDeploymentStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	deploymentId := c.Param("deploymentId")
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "deploymentId is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.GetStackDeploymentStatus(uuid.MustParse(deploymentId))
	if err != nil {
		logger.Error("failed to get stack deployment status", zap.Error(err), zap.String("id", id), zap.String("deploymentId", deploymentId))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) GetStackByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.GetStackByID(uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to get stack by id", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) InstallBridge(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.InstallBridge(c, id)
	if err != nil {
		logger.Error("failed to install bridge", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) UninstallBridge(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.UninstallBridge(c, id)
	if err != nil {
		logger.Error("failed to uninstall bridge", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) UninstallBlockExplorer(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.UninstallBlockExplorer(c, id)
	if err != nil {
		logger.Error("failed to uninstall block explorer", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) InstallBlockExplorer(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	var request dtos.InstallBlockExplorerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.InstallBlockExplorer(c, id, request)
	if err != nil {
		logger.Error("failed to install block explorer", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

func NewThanosHandler(server *servers.Server) *ThanosDeploymentHandler {
	deploymentRepo := postgresRepositories.NewDeploymentRepository(server.PostgresDB)
	stackRepo := postgresRepositories.NewStackRepository(server.PostgresDB)
	integrationRepo := postgresRepositories.NewIntegrationRepository(server.PostgresDB)

	taskManager := taskmanager.NewTaskManager(5, 20)

	return &ThanosDeploymentHandler{
		ThanosDeploymentService: services.NewThanosService(deploymentRepo, stackRepo, integrationRepo, taskManager),
	}
}
