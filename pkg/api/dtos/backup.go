package dtos

type BackupRequest struct {
	Limit string `json:"limit"`
}

type BackupSnapshotRequest struct {
	AwsAccessKey *string `json:"awsAccessKey,omitempty"`
	AwsSecretKey *string `json:"awsSecretAccessKey,omitempty"`
	AwsRegion    *string `json:"awsRegion,omitempty"`
}

type BackupAttachRequest struct {
	EfsId        *string `json:"efsId"`
	Pvcs         *string `json:"pvcs"`
	Stss         *string `json:"stss"`
	AwsAccessKey *string `json:"awsAccessKey,omitempty"`
	AwsSecretKey *string `json:"awsSecretAccessKey,omitempty"`
	AwsRegion    *string `json:"awsRegion,omitempty"`
}

type BackupConfigureRequest struct {
	Daily        *string `json:"daily"`
	Keep         *string `json:"keep"`
	Reset        *bool   `json:"reset"`
	AwsAccessKey *string `json:"awsAccessKey,omitempty"`
	AwsSecretKey *string `json:"awsSecretAccessKey,omitempty"`
	AwsRegion    *string `json:"awsRegion,omitempty"`
}

type BackupRestoreRequest struct {
	RecoveryPointID string  `json:"recoveryPointID"`
	AwsAccessKey    *string `json:"awsAccessKey,omitempty"`
	AwsSecretKey    *string `json:"awsSecretAccessKey,omitempty"`
	AwsRegion       *string `json:"awsRegion,omitempty"`
}
