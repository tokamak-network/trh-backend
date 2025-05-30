package handlers

import (
	"net/http"

	"trh-backend/pkg/application/services"
	thanosDomainServices "trh-backend/pkg/domain/services"
	"trh-backend/pkg/interfaces/api/dtos"
	"trh-backend/pkg/interfaces/api/servers"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ThanosHandler struct {
	ThanosService *services.ThanosService
}

func (h *ThanosHandler) DeployThanos(c *gin.Context) {
	var request dtos.DeployThanosRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.ThanosService.ValidateThanosRequest(request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stackId, err := h.ThanosService.CreateThanosStack(request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OK", "stackId": stackId})
}

func (h *ThanosHandler) TerminateThanos(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	err := h.ThanosService.TerminateThanosStack(uuid.MustParse(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func (h *ThanosHandler) ResumeThanos(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	err := h.ThanosService.ResumeThanosStack(uuid.MustParse(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func (h *ThanosHandler) GetAllStacks(c *gin.Context) {
	stacks, err := h.ThanosService.GetAllStacks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stacks": stacks})
}

func (h *ThanosHandler) GetStackStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	status, err := h.ThanosService.GetStackStatus(uuid.MustParse(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

func (h *ThanosHandler) GetStackDeployments(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	deployments, err := h.ThanosService.GetStackDeployments(uuid.MustParse(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deployments": deployments})
}

func (h *ThanosHandler) GetStackDeployment(c *gin.Context) {
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
	deployment, err := h.ThanosService.GetStackDeployment(uuid.MustParse(id), uuid.MustParse(deploymentId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deployment": deployment})
}

func (h *ThanosHandler) GetStackDeploymentStatus(c *gin.Context) {
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
	status, err := h.ThanosService.GetStackDeploymentStatus(uuid.MustParse(deploymentId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

func (h *ThanosHandler) GetStackByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	stack, err := h.ThanosService.GetStackByID(uuid.MustParse(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stack": stack})
}

func NewThanosHandler(server *servers.Server) *ThanosHandler {
	thanosDomainService := thanosDomainServices.NewThanosDomainService()
	return &ThanosHandler{
		ThanosService: services.NewThanosService(server.PostgresDB, thanosDomainService),
	}
}
