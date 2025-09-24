package dtos

type BackupRequest struct {
	Limit string `json:"limit"`
}

type BackupAttachRequest struct {
	EfsId *string `json:"efsId"`
	Pvcs  *string `json:"pvcs"`
	Stss  *string `json:"stss"`
}

type BackupConfigureRequest struct {
	Daily *string `json:"daily"`
	Keep  *string `json:"keep"`
	Reset *bool   `json:"reset"`
}
