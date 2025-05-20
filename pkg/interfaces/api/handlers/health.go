package handlers

import (
	"net/http"

	"trh-backend/pkg/interfaces/api/servers"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	Server *servers.Server
}

func (h *HealthHandler) GetHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func NewHealthHandler(server *servers.Server) *HealthHandler {
	return &HealthHandler{
		Server: server,
	}
}
