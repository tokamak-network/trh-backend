package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"go.uber.org/zap"
)

// FileDownloadConfig holds configuration for file download
type FileDownloadConfig struct {
	FilePath    string
	Filename    string
	ContentType string
}

// FileDownloadResult holds the result of file download preparation
type FileDownloadResult struct {
	File        *os.File
	Filename    string
	ContentType string
	Size        int64
}

// PrepareFileDownload validates and prepares a file for download
func PrepareFileDownload(ctx context.Context, config FileDownloadConfig) (*FileDownloadResult, error) {
	// Validate file path
	if config.FilePath == "" {
		return nil, fmt.Errorf("file path is required")
	}

	// Check if file exists
	if _, err := os.Stat(config.FilePath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", config.FilePath)
		}
		logger.Error("failed to stat file", zap.String("path", config.FilePath), zap.Error(err))
		return nil, fmt.Errorf("failed to access file: %w", err)
	}

	// Open the file
	file, err := os.Open(config.FilePath)
	if err != nil {
		logger.Error("failed to open file", zap.String("path", config.FilePath), zap.Error(err))
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Get file info for setting headers
	fileInfo, err := file.Stat()
	if err != nil {
		file.Close() // Clean up on error
		logger.Error("failed to get file info", zap.String("path", config.FilePath), zap.Error(err))
		return nil, fmt.Errorf("failed to get file info: %w", err)
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

	return &FileDownloadResult{
		File:        file,
		Filename:    filename,
		ContentType: contentType,
		Size:        fileInfo.Size(),
	}, nil
}
