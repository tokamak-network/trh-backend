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

// @Summary      Deploy Thanos Stack
// @Description  Deploy Thanos Stack
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        request  body      dtos.DeployThanosRequest  true  "Deploy Thanos Stack Request"
// @Success      200      {object}  entities.Response
// @Router       /stacks/thanos [post]
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

	if request.RegisterCandidate {
		if request.RegisterCandidateParams == nil {
			c.JSON(http.StatusBadRequest, &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "registerCandidateParams is required",
				Data:    nil,
			})
			return
		}
	} else {
		request.RegisterCandidateParams = nil
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

// @Summary      Stop Thanos Stack
// @Description  Stop Thanos Stack
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
// @Success      200      {object}  entities.Response
// @Router       /stacks/thanos/{id}/stop [post]
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

// @Summary      Update Network
// @Description  Update Network
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
// @Param        request  body      dtos.UpdateNetworkRequest  true  "Update Network Request"
// @Success      200      {object}  entities.Response
// @Router       /stacks/thanos/{id} [put]
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

// @Summary      Terminate Thanos Stack
// @Description  Terminate Thanos Stack
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
// @Success      200      {object}  entities.Response
// @Router       /stacks/thanos/{id} [delete]
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

// @Summary      Resume Thanos Stack
// @Description  Resume Thanos Stack
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
// @Success      200      {object}  entities.Response
// @Router       /stacks/thanos/{id}/resume [post]
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

// @Summary      Get All Stacks
// @Description  Get All Stacks
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Success      200      {object}  entities.Response
// @Router       /stacks/thanos [get]
func (h *ThanosDeploymentHandler) GetAllStacks(c *gin.Context) {
	response, err := h.ThanosDeploymentService.GetAllStacks()
	if err != nil {
		logger.Error("failed to get all stacks", zap.Error(err))
	}
	c.JSON(int(response.Status), response)
}

// @Summary      Get Stack Status
// @Description  Get Stack Status
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
// @Success      200      {object}  entities.Response
// @Router       /stacks/thanos/{id} [get]
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

// @Summary      Get Deployments
// @Description  Get Deployments
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
// @Success      200      {object}  entities.Response
// @Router       /stacks/thanos/{id}/deployments [get]
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

// @Summary      Get Integrations
// @Description  Get Integrations
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
// @Success      200      {object}  entities.Response
// @Router       /stacks/thanos/{id}/integrations [get]
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

// @Summary      Get Integration By ID
// @Description  Get Integration By ID
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
// @Param        integrationId   path      string  true  "Integration ID"
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

// @Summary      Get Stack Deployment
// @Description  Get Stack Deployment
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
// @Param        deploymentId   path      string  true  "Deployment ID"
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

// @Summary      Get Stack Deployment Status
// @Description  Get Stack Deployment Status
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
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

// @Summary      Get Stack By ID
// @Description  Get Stack By ID
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
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

// @Summary      Install Bridge
// @Description  Install Bridge
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
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

// @Summary      Uninstall Bridge
// @Description  Uninstall Bridge
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
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

// @Summary      Uninstall Block Explorer
// @Description  Uninstall Block Explorer
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
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

// @Summary      Install Block Explorer
// @Description  Install Block Explorer
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
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

// @Summary      Install Monitoring
// @Description  Install Monitoring
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
func (h *ThanosDeploymentHandler) InstallMonitoring(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	var request dtos.InstallMonitoringRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.InstallMonitoring(c.Request.Context(), uuid.MustParse(id), request)
	if err != nil {
		logger.Error("failed to install monitoring", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary      Uninstall Monitoring

// @Description  Uninstall Monitoring
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
func (h *ThanosDeploymentHandler) UninstallMonitoring(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.UninstallMonitoring(c.Request.Context(), uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to uninstall monitoring", zap.Error(err), zap.String("id", id))
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
