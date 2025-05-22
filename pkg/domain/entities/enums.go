package entities

type DeploymentNetwork string

const (
	DeploymentNetworkMainnet     DeploymentNetwork = "Mainnet"
	DeploymentNetworkTestnet     DeploymentNetwork = "Testnet"
	DeploymentNetworkLocalDevnet DeploymentNetwork = "LocalDevnet"
)

type Status string

const (
	StatusActive      Status = "Active"
	StatusInactive    Status = "Inactive"
	StatusCreating    Status = "Creating"
	StatusUpdating    Status = "Updating"
	StatusTerminating Status = "Terminating"
)

type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "Pending"
	DeploymentStatusInProgress DeploymentStatus = "InProgress"
	DeploymentStatusFailed     DeploymentStatus = "Failed"
	DeploymentStatusStopped    DeploymentStatus = "Stopped"
	DeploymentStatusCompleted  DeploymentStatus = "Completed"
)
