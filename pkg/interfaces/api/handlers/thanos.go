package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"trh-backend/pkg/domain/repositories"
	postgresRepositories "trh-backend/pkg/infrastructure/postgres/repositories"
	"trh-backend/pkg/interfaces/api/dtos"
	"trh-backend/pkg/interfaces/api/servers"
)

type ThanosHandler struct {
	StackRepository repositories.StackRepository
}

func (h *ThanosHandler) DeployThanos(c *gin.Context) {
	var request dtos.DeployThanosRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stack, err := h.StackRepository.CreateStack(request)
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
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func NewThanosHandler(server *servers.Server) *ThanosHandler {
	stackRepository := postgresRepositories.NewStackPostgresRepository(server.PostgresDB)
	return &ThanosHandler{
		StackRepository: stackRepository,
	}
}
