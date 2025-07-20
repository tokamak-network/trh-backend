package thanos

import (
	"net/http"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

// @Summary		Get Deployments
// @Description	Get Deployments
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/deployments [get]
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

// @Summary		Get Stack Deployment
// @Description	Get Stack Deployment
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id				path		string	true	"Thanos Stack ID"
// @Param			deploymentId	path		string	true	"Deployment ID"
// @Success		200				{object}	entities.Response
// @Router			/stacks/thanos/{id}/deployments/{deploymentId} [get]
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

// @Summary		Get Stack Deployment Status
// @Description	Get Stack Deployment Status
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id				path		string	true	"Thanos Stack ID"
// @Param			deploymentId	path		string	true	"Deployment ID"
// @Success		200				{object}	entities.Response
// @Router			/stacks/thanos/{id}/deployments/{deploymentId}/status [get]
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
