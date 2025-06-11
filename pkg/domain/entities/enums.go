package entities

type DeploymentNetwork string

const (
	DeploymentNetworkMainnet     DeploymentNetwork = "Mainnet"
	DeploymentNetworkTestnet     DeploymentNetwork = "Testnet"
	DeploymentNetworkLocalDevnet DeploymentNetwork = "LocalDevnet"
)

type StackStatus string

const (
	StackStatusPending           StackStatus = "Pending"
	StackStatusDeployed          StackStatus = "Deployed"
	StackStatusStopped           StackStatus = "Stopped"
	StackStatusDeploying         StackStatus = "Deploying"
	StackStatusUpdating          StackStatus = "Updating"
	StackStatusTerminating       StackStatus = "Terminating"
	StackStatusTerminated        StackStatus = "Terminated"
	StackStatusFailedToDeploy    StackStatus = "FailedToDeploy"
	StackStatusFailedToUpdate    StackStatus = "FailedToUpdate"
	StackStatusFailedToTerminate StackStatus = "FailedToTerminate"
	StackStatusUnknown           StackStatus = "Unknown"
)

type DeploymentStatus string

const (
	DeploymentStatusPending     DeploymentStatus = "Pending"
	DeploymentStatusInProgress  DeploymentStatus = "InProgress"
	DeploymentStatusFailed      DeploymentStatus = "Failed"
	DeploymentStatusStopped     DeploymentStatus = "Stopped"
	DeploymentStatusCompleted   DeploymentStatus = "Completed"
	DeploymentStatusTerminating DeploymentStatus = "Terminating"
	DeploymentStatusTerminated  DeploymentStatus = "Terminated"
	DeploymentStatusUnknown     DeploymentStatus = "Unknown"
)
