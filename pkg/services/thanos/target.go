package thanos

import "github.com/tokamak-network/trh-backend/pkg/domain/entities"

// isLocalTarget determines whether a given network type implies a local deployment target.
// This is the single decision point for DTO-level code that doesn't yet have a Target field.
// When the DTO gains an explicit Target field, update this function accordingly.
func isLocalTarget(network entities.DeploymentNetwork) bool {
	return network == entities.DeploymentNetworkLocalTestnet
}
