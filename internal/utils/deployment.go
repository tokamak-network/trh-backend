package utils

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

func GetDeploymentPath(
	stack string,
	network entities.DeploymentNetwork,
	deploymentID string,
) string {
	rootDir, _ := os.Getwd()
	return path.Join(rootDir, "storage", "deployments", stack, string(network), deploymentID)
}

func GetLogPath(
	stackID uuid.UUID,
	plugin string,
) string {
	rootDir, _ := os.Getwd()
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	return path.Join(rootDir, "storage", "logs", stackID.String(), timestamp+fmt.Sprintf("_%s_logs.txt", plugin))
}
