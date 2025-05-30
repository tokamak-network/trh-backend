package entities

type DeploymentNetwork string

const (
	DeploymentNetworkMainnet     DeploymentNetwork = "Mainnet"
	DeploymentNetworkTestnet     DeploymentNetwork = "Testnet"
	DeploymentNetworkLocalDevnet DeploymentNetwork = "LocalDevnet"
)

type Status string

const (
	StatusPending           Status = "Pending"
	StatusDeployed          Status = "Deployed"
	StatusDeploying         Status = "Deploying"
	StatusUpdating          Status = "Updating"
	StatusTerminating       Status = "Terminating"
	StatusTerminated        Status = "Terminated"
	StatusFailedToDeploy    Status = "FailedToDeploy"
	StatusFailedToUpdate    Status = "FailedToUpdate"
	StatusFailedToTerminate Status = "FailedToTerminate"
	StatusUnknown           Status = "Unknown"
)

type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "Pending"
	DeploymentStatusInProgress DeploymentStatus = "InProgress"
	DeploymentStatusFailed     DeploymentStatus = "Failed"
	DeploymentStatusStopped    DeploymentStatus = "Stopped"
	DeploymentStatusCompleted  DeploymentStatus = "Completed"
	DeploymentStatusUnknown    DeploymentStatus = "Unknown"
)
