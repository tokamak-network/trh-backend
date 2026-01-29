package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// ZipDirectory compresses a directory into a zip file at destPath.
func ZipDirectory(sourceDir, destPath string) error {
	if sourceDir == "" {
		return fmt.Errorf("source directory is required")
	}
	if destPath == "" {
		return fmt.Errorf("destination path is required")
	}
	info, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to stat source directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source path is not a directory: %s", sourceDir)
	}

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer destFile.Close()

	zipWriter := zip.NewWriter(destFile)
	defer zipWriter.Close()

	return filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to resolve relative path: %w", err)
		}
		zipPath := filepath.ToSlash(relPath)
		entry, err := zipWriter.Create(zipPath)
		if err != nil {
			return fmt.Errorf("failed to create zip entry: %w", err)
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		if _, err := io.Copy(entry, file); err != nil {
			return fmt.Errorf("failed to write zip entry: %w", err)
		}
		return nil
	})
}
