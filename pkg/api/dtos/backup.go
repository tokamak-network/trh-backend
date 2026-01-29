package dtos

type BackupRequest struct {
	Limit string `json:"limit"`
}

type BackupAttachRequest struct {
	EfsId        *string `json:"efsId"`
	Pvcs         *string `json:"pvcs"`
	Stss         *string `json:"stss"`
	BackupPvPvc  *bool   `json:"backupPvPvc,omitempty"`
	AwsAccessKey *string `json:"awsAccessKey,omitempty"`
	AwsSecretKey *string `json:"awsSecretAccessKey,omitempty"`
	AwsRegion    *string `json:"awsRegion,omitempty"`
}

type BackupConfigureRequest struct {
	Daily *string `json:"daily"`
	Keep  *string `json:"keep"`
	Reset *bool   `json:"reset"`
}

type BackupRestoreRequest struct {
	RecoveryPointID string  `json:"recoveryPointID"`
	Attach          *bool   `json:"attach,omitempty"`
	Pvcs            *string `json:"pvcs,omitempty"`
	Stss            *string `json:"stss,omitempty"`
	AwsAccessKey    *string `json:"awsAccessKey,omitempty"`
	AwsSecretKey    *string `json:"awsSecretAccessKey,omitempty"`
	AwsRegion       *string `json:"awsRegion,omitempty"`
}
