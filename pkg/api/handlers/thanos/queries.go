package thanos

import (
	"fmt"
	"io"
	"net/http"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

// @Summary		Get All Stacks
// @Description	Get All Stacks (Authenticated users)
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Success		200	{object}	entities.Response
// @Failure		401	{object}	map[string]interface{}
// @Router			/stacks/thanos [get]
func (h *ThanosDeploymentHandler) GetAllStacks(c *gin.Context) {
	response, err := h.ThanosDeploymentService.GetAllStacks()
	if err != nil {
		logger.Error("failed to get all stacks", zap.Error(err))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Get Stack Status
// @Description	Get Stack Status
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/status [get]
func (h *ThanosDeploymentHandler) GetStackStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.GetStackStatus(uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to get stack status", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Get Stack By ID
// @Description	Get Stack By ID
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id} [get]
func (h *ThanosDeploymentHandler) GetStackByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.GetStackByID(uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to get stack by id", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary     Download Rollup Config File
// @Description Download the rollup config file for a stack
// @Tags        Thanos Stack
// @Accept      json
// @Produce     application/json
// @Param       id path string true "Thanos Stack ID"
// @Success     200 {file} file "Rollup config file content"
// @Failure     400 {object} entities.Response
// @Failure     404 {object} entities.Response
// @Failure     500 {object} entities.Response
// @Router      /stacks/thanos/{id}/rollupconfig [get]
func (h *ThanosDeploymentHandler) DownloadRollupConfig(c *gin.Context) {
	stackIdStr := c.Param("id")
	if stackIdStr == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	stackId, err := uuid.Parse(stackIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack ID format",
			Data:    nil,
		})
		return
	}

	// Get rollup config file path
	filePath, err := h.ThanosDeploymentService.GetRollupConfigFilePath(stackId)
	if err != nil {
		logger.Error("failed to get rollup config file path",
			zap.String("stackId", stackIdStr),
			zap.Error(err))

		var statusCode int
		if err.Error() == "stack not found" {
			statusCode = http.StatusNotFound
		} else if err.Error() == "stack metadata not found" ||
			err.Error() == "rollup config file not available for this stack" ||
			err.Error() == "rollup config file not found on filesystem" {
			statusCode = http.StatusNotFound
		} else {
			statusCode = http.StatusInternalServerError
		}

		c.JSON(statusCode, &entities.Response{
			Status:  uint64(statusCode),
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	// Generate filename based on stack ID
	filename := fmt.Sprintf("rollup-config_%s.json", stackId.String()[:8])

	// Prepare file for download
	result, err := utils.PrepareFileDownload(c.Request.Context(), utils.FileDownloadConfig{
		FilePath:    filePath,
		Filename:    filename,
		ContentType: "application/json",
	})
	if err != nil {
		logger.Error("failed to prepare file download", zap.Error(err))
		c.JSON(http.StatusInternalServerError, &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "failed to prepare file for download",
			Data:    nil,
		})
		return
	}
	defer result.File.Close()

	// Set headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", result.Filename))
	c.Header("Content-Type", result.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", result.Size))

	// Stream the file content
	_, err = io.Copy(c.Writer, result.File)
	if err != nil {
		logger.Error("failed to stream file", zap.Error(err))
		// At this point headers are already sent, so we can't send a JSON error response
		return
	}
}
