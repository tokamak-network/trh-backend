package utils

import (
	"os"
	"path"
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
	return path.Join(rootDir, "storage", "logs", stackID.String(), deploymentID.String(), "logs.txt")
}
