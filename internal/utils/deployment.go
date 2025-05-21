package utils

import (
	"path"

	"trh-backend/pkg/infrastructure/postgres/schemas"
)

func GetDeploymentPath(
	stack string,
	network schemas.DeploymentNetwork,
	deploymentID string,
) string {
	return path.Join("storage", "deployments", stack, string(network), deploymentID)
}
