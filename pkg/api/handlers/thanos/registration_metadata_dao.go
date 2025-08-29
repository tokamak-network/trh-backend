package thanos

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"go.uber.org/zap"
)

// @Summary		Register Metadata DAO
// @Description	Register Metadata DAO
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/register-metadata-dao [post]
func (h *ThanosDeploymentHandler) RegisterMetadataDAO(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
	}

	var request dtos.RegisterMetadataDAORequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	r, err := request.Validate(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	if r != nil {
		request = *r
	}

	response, err := h.ThanosDeploymentService.RegisterMetadataDAO(c, uuid.MustParse(id), request)
	if err != nil {
		logger.Error("failed to register metadata dao", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) GetRegisterMetadataDAO(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.GetRegisterMetadataDAO(c, uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to get register metadata dao", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}
