// Package types defines the data structures for KASA device responses.
package types

// CloudInfo represents cloud connectivity information for a device.
type CloudInfo struct {
	Binded        int    `json:"binded"`         // 1 = bound to cloud account
	CldConnection int    `json:"cld_connection"` // 1 = connected to cloud
	FwDlPage      string `json:"fwDlPage"`       // Firmware download page URL
	FwNotifyType  int    `json:"fwNotifyType"`   // Firmware notification type
	IllegalType   int    `json:"illegalType"`    // Illegal device flag
	Server        string `json:"server"`         // Cloud server hostname
	TCSpStatus    int    `json:"tcspStatus"`     // TC SP status
	TCSpInfo      string `json:"tcspInfo"`       // TC SP info
	Username      string `json:"username"`       // Cloud account username
	ErrCode       int    `json:"err_code"`       // Error code
	ErrMsg        string `json:"err_msg"`        // Error message
}

// CloudInfoResponse wraps the get_info response from cnCloud namespace.
type CloudInfoResponse struct {
	CloudInfo struct {
		GetInfo CloudInfo `json:"get_info"`
	} `json:"cnCloud"`
}

// IsConnected returns true if the device is connected to the cloud.
func (c *CloudInfo) IsConnected() bool {
	return c.CldConnection == 1
}

// IsBound returns true if the device is bound to a cloud account.
func (c *CloudInfo) IsBound() bool {
	return c.Binded == 1
}
