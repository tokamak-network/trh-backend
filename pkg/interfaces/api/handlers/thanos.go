package handlers

import (
	"net/http"

	"trh-backend/pkg/application/services"
	postgresRepositories "trh-backend/pkg/infrastructure/postgres/repositories"
	"trh-backend/pkg/interfaces/api/dtos"
	"trh-backend/pkg/interfaces/api/servers"

	"github.com/gin-gonic/gin"
)

type ThanosHandler struct {
	StackService *services.StackService
}

func (h *ThanosHandler) DeployThanos(c *gin.Context) {
	var request dtos.DeployThanosRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stack, err := h.StackService.DeployStack(request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OK", "stack": stack})
}

func (h *ThanosHandler) DestroyThanos(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	err := h.StackService.DestroyStack(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func NewThanosHandler(server *servers.Server) *ThanosHandler {
	stackRepository := postgresRepositories.NewStackPostgresRepository(server.PostgresDB)
	return &ThanosHandler{
		StackService: services.NewStackService(stackRepository),
	}
}
