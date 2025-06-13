package handlers

import (
	"net/http"

	"github.com/tokamak-network/trh-backend/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/api/servers"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := request.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	request.AdminAccount = utils.TrimPrivateKey(request.AdminAccount)
	request.SequencerAccount = utils.TrimPrivateKey(request.SequencerAccount)
	request.BatcherAccount = utils.TrimPrivateKey(request.BatcherAccount)
	request.ProposerAccount = utils.TrimPrivateKey(request.ProposerAccount)

	stackId, err := h.ThanosDeploymentService.CreateThanosStack(c, request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OK", "stackId": stackId})
}

func (h *ThanosDeploymentHandler) Stop(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	err := h.ThanosDeploymentService.StopDeployingThanosStack(c, uuid.MustParse(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func (h *ThanosDeploymentHandler) UpdateNetwork(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	var request dtos.UpdateNetworkRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.ThanosDeploymentService.UpdateNetwork(c, uuid.MustParse(id), request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func (h *ThanosDeploymentHandler) Terminate(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	err := h.ThanosDeploymentService.TerminateThanosStack(c, uuid.MustParse(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func (h *ThanosDeploymentHandler) Resume(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	err := h.ThanosDeploymentService.ResumeThanosStack(c, uuid.MustParse(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func (h *ThanosDeploymentHandler) GetAllStacks(c *gin.Context) {
	stacks, err := h.ThanosDeploymentService.GetAllStacks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stacks": stacks})
}

func (h *ThanosDeploymentHandler) GetStackStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	status, err := h.ThanosDeploymentService.GetStackStatus(uuid.MustParse(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

func (h *ThanosDeploymentHandler) GetDeployments(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	deployments, err := h.ThanosDeploymentService.GetDeployments(uuid.MustParse(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deployments": deployments})
}

func (h *ThanosDeploymentHandler) GetIntegrations(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	integrations, err := h.ThanosDeploymentService.GetIntegrations(uuid.MustParse(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"integrations": integrations})
}

func (h *ThanosDeploymentHandler) GetIntegrationById(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	integrationId := c.Param("integrationId")
	if integrationId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "integrationId is required"})
		return
	}
	integration, err := h.ThanosDeploymentService.GetIntegration(
		uuid.MustParse(id),
		uuid.MustParse(integrationId),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"integration": integration})
}

func (h *ThanosDeploymentHandler) GetStackDeployment(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	deploymentId := c.Param("deploymentId")
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deploymentId is required"})
		return
	}
	deployment, err := h.ThanosDeploymentService.GetStackDeployment(
		uuid.MustParse(id),
		uuid.MustParse(deploymentId),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deployment": deployment})
}

func (h *ThanosDeploymentHandler) GetStackDeploymentStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	deploymentId := c.Param("deploymentId")
	if deploymentId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deploymentId is required"})
		return
	}
	status, err := h.ThanosDeploymentService.GetStackDeploymentStatus(uuid.MustParse(deploymentId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

func (h *ThanosDeploymentHandler) GetStackByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	stack, err := h.ThanosDeploymentService.GetStackByID(uuid.MustParse(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stacks": stack})
}

func (h *ThanosDeploymentHandler) InstallBridge(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	err := h.ThanosDeploymentService.InstallBridge(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func (h *ThanosDeploymentHandler) UninstallBridge(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	err := h.ThanosDeploymentService.UninstallBridge(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func (h *ThanosDeploymentHandler) UninstallBlockExplorer(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	err := h.ThanosDeploymentService.UninstallBlockExplorer(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func (h *ThanosDeploymentHandler) InstallBlockExplorer(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	var request dtos.InstallBlockExplorerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.ThanosDeploymentService.InstallBlockExplorer(c, id, request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
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
