package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"go.uber.org/zap"
)

// FileDownloadConfig holds configuration for file download
type FileDownloadConfig struct {
	FilePath    string
	Filename    string
	ContentType string
}

// DownloadFile streams a file to the HTTP response with proper headers
func DownloadFile(c *gin.Context, config FileDownloadConfig) {
	// Validate file path
	if config.FilePath == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "file path is required",
			Data:    nil,
		})
		return
	}

	// Check if file exists
	if _, err := os.Stat(config.FilePath); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, &entities.Response{
				Status:  http.StatusNotFound,
				Message: "file not found",
				Data:    nil,
			})
			return
		}
		logger.Error("failed to stat file", zap.String("path", config.FilePath), zap.Error(err))
		c.JSON(http.StatusInternalServerError, &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "failed to access file",
			Data:    nil,
		})
		return
	}

	// Open the file
	file, err := os.Open(config.FilePath)
	if err != nil {
		logger.Error("failed to open file", zap.String("path", config.FilePath), zap.Error(err))
		c.JSON(http.StatusInternalServerError, &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "failed to open file",
			Data:    nil,
		})
		return
	}
	defer file.Close()

	// Get file info for setting headers
	fileInfo, err := file.Stat()
	if err != nil {
		logger.Error("failed to get file info", zap.String("path", config.FilePath), zap.Error(err))
		c.JSON(http.StatusInternalServerError, &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "failed to get file info",
			Data:    nil,
		})
		return
	}

	// Use provided filename or extract from path
	filename := config.Filename
	if filename == "" {
		filename = filepath.Base(config.FilePath)
	}

	// Set default content type if not provided
	contentType := config.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Set headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Stream the file content
	_, err = io.Copy(c.Writer, file)
	if err != nil {
		logger.Error("failed to stream file", zap.String("path", config.FilePath), zap.Error(err))
		// At this point headers are already sent, so we can't send a JSON error response
		return
	}
}
