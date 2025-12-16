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

var DeploymentStatusesAllowedToInstall = []DeploymentStatus{
	DeploymentStatusFailed,     // When failed, we can try to install again
	DeploymentStatusStopped,    // When stopped, we can try to install again
	DeploymentStatusTerminated, // When terminated, we can try to install again
	DeploymentStatusCancelled,  // When cancelled, we can try to install again
	DeploymentStatusUnknown,    // When unknown, we can try to install again
}

const (
	DeploymentStatusPending     DeploymentStatus = "Pending"
	DeploymentStatusInProgress  DeploymentStatus = "InProgress"
	DeploymentStatusFailed      DeploymentStatus = "Failed"
	DeploymentStatusStopped     DeploymentStatus = "Stopped"
	DeploymentStatusCompleted   DeploymentStatus = "Completed"
	DeploymentStatusTerminating DeploymentStatus = "Terminating"
	DeploymentStatusTerminated  DeploymentStatus = "Terminated"
	DeploymentStatusCancelling  DeploymentStatus = "Cancelling"
	DeploymentStatusCancelled   DeploymentStatus = "Cancelled"
	DeploymentStatusUnknown     DeploymentStatus = "Unknown"
)

// DeploymentRunStatus is used for deployment steps (not integrations)
type DeploymentRunStatus string

const (
	DeploymentRunStatusPending    DeploymentRunStatus = "Pending"
	DeploymentRunStatusInProgress DeploymentRunStatus = "InProgress"
	DeploymentRunStatusFailed     DeploymentRunStatus = "Failed"
	DeploymentRunStatusSuccess    DeploymentRunStatus = "Success"
	DeploymentRunStatusStopped    DeploymentRunStatus = "Stopped"
	DeploymentRunStatusCancelled  DeploymentRunStatus = "Cancelled"
)

type UserRole string

const (
	UserRoleAdmin UserRole = "Admin"
	UserRoleUser  UserRole = "User"
)
