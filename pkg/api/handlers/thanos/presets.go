package thanos

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/services/thanos/presets"
	"gorm.io/gorm"
)

// @Summary		List Presets
// @Description	Return all available deployment presets
// @Tags			Thanos Presets
// @Produce		json
// @Security		BearerAuth
// @Success		200	{object}	entities.Response
// @Failure		401	{object}	map[string]interface{}
// @Router			/stacks/thanos/presets [get]
func (h *ThanosDeploymentHandler) ListPresets(c *gin.Context) {
	svc := presets.NewService()
	defs := svc.ListAll()

	c.JSON(http.StatusOK, &entities.Response{
		Status:  http.StatusOK,
		Message: "ok",
		Data:    defs,
	})
}

// @Summary		Get Preset
// @Description	Return a single deployment preset by ID
// @Tags			Thanos Presets
// @Produce		json
// @Security		BearerAuth
// @Param			presetId	path		string	true	"Preset ID"
// @Success		200			{object}	entities.Response
// @Failure		401			{object}	map[string]interface{}
// @Failure		404			{object}	map[string]interface{}
// @Router			/stacks/thanos/presets/{presetId} [get]
func (h *ThanosDeploymentHandler) GetPresetByID(c *gin.Context) {
	presetID := c.Param("presetId")

	svc := presets.NewService()
	def, err := svc.GetByID(presetID)
	if err != nil {
		c.JSON(http.StatusNotFound, &entities.Response{
			Status:  http.StatusNotFound,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, &entities.Response{
		Status:  http.StatusOK,
		Message: "ok",
		Data:    def,
	})
}

// @Summary		Get Funding Status
// @Description	Return L1 funding status for each role account of the given stack
// @Tags			Thanos Presets
// @Produce		json
// @Security		BearerAuth
// @Param			id	path		string	true	"Stack ID"
// @Success		200	{object}	entities.Response
// @Failure		400	{object}	map[string]interface{}
// @Failure		401	{object}	map[string]interface{}
// @Failure		404	{object}	map[string]interface{}
// @Router			/stacks/thanos/{id}/funding-status [get]
func (h *ThanosDeploymentHandler) GetFundingStatus(c *gin.Context) {
	stackID := c.Param("id")
	if stackID == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
		})
		return
	}

	result, err := h.ThanosDeploymentService.GetFundingStatus(c.Request.Context(), stackID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, &entities.Response{
				Status:  http.StatusNotFound,
				Message: "stack not found",
			})
			return
		}
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, &entities.Response{
		Status:  http.StatusOK,
		Message: "ok",
		Data:    result,
	})
}
