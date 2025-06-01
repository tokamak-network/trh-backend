package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct{}

func (h *HealthHandler) GetHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}
