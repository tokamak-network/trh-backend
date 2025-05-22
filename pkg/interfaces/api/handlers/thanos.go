package handlers

import (
	"net/http"

	"trh-backend/pkg/application/services"
	thanosDomainServices "trh-backend/pkg/domain/services"
	"trh-backend/pkg/interfaces/api/dtos"
	"trh-backend/pkg/interfaces/api/servers"

	"github.com/gin-gonic/gin"
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

	stackId, err := h.ThanosService.DeployThanosStack(request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OK", "stackId": stackId})
}

func (h *ThanosHandler) DestroyThanos(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	err := h.ThanosService.DestroyThanosStack(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func NewThanosHandler(server *servers.Server) *ThanosHandler {
	thanosDomainService := thanosDomainServices.NewThanosDomainService()
	return &ThanosHandler{
		ThanosService: services.NewThanosService(server.PostgresDB, thanosDomainService),
	}
}
