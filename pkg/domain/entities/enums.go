package entities

type DeploymentNetwork string

const (
	DeploymentNetworkMainnet     DeploymentNetwork = "Mainnet"
	DeploymentNetworkTestnet     DeploymentNetwork = "Testnet"
	DeploymentNetworkLocalDevnet DeploymentNetwork = "LocalDevnet"
)

type StackStatus string

const (
	StatusPending           StackStatus = "Pending"
	StatusDeployed          StackStatus = "Deployed"
	StatusDeploying         StackStatus = "Deploying"
	StatusUpdating          StackStatus = "Updating"
	StatusTerminating       StackStatus = "Terminating"
	StatusTerminated        StackStatus = "Terminated"
	StatusFailedToDeploy    StackStatus = "FailedToDeploy"
	StatusFailedToUpdate    StackStatus = "FailedToUpdate"
	StatusFailedToTerminate StackStatus = "FailedToTerminate"
	StatusUnknown           StackStatus = "Unknown"
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
