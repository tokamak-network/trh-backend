package utils

import (
	"os"
	"path"
	"time"
	"trh-backend/pkg/domain/entities"

	"github.com/google/uuid"
)

func GetDeploymentPath(
	stack string,
	network entities.DeploymentNetwork,
	deploymentID string,
) string {
	rootDir, _ := os.Getwd()
	return path.Join(rootDir, "storage", "deployments", stack, string(network), deploymentID)
}

func GetDeploymentLogPath(
	stackID uuid.UUID,
	deploymentID uuid.UUID,
) string {
	rootDir, _ := os.Getwd()
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	return path.Join(rootDir, "storage", "logs", stackID.String(), deploymentID.String(), timestamp+"_logs.txt")
}

func GetDestroyLogPath(
	stackID uuid.UUID,
) string {
	rootDir, _ := os.Getwd()
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	return path.Join(rootDir, "storage", "logs", stackID.String(), timestamp+"_destroy_logs.txt")
}
