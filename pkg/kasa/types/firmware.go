// Package types defines the data structures for KASA device responses.
package types

// FirmwareInfo represents information about an available firmware update.
type FirmwareInfo struct {
	FWVer       string `json:"fwVer"`       // Firmware version
	ReleaseDate string `json:"releaseDate"` // Release date
	ReleaseNote string `json:"releaseNote"` // Release notes
	DownloadURL string `json:"downloadURL"` // Download URL
	ObjURL      string `json:"objUrl"`      // Object URL
	Type        int    `json:"type"`        // Firmware type
}

// AvailableFirmwareResponse wraps the get_available_firmwares response.
type AvailableFirmwareResponse struct {
	System struct {
		GetAvailableFirmwares struct {
			FWList  []FirmwareInfo `json:"fwList"`
			ErrCode int            `json:"err_code"`
			ErrMsg  string         `json:"err_msg,omitempty"`
		} `json:"get_available_firmwares"`
	} `json:"system"`
}

// DownloadState represents the firmware download status.
type DownloadState struct {
	Status  int `json:"status"`   // 0 = idle, 1 = downloading, 2 = done
	Ratio   int `json:"ratio"`    // Download percentage (0-100)
	ErrCode int `json:"err_code"`
	ErrMsg  string `json:"err_msg,omitempty"`
}

// StatusString returns a human-readable status string.
func (d *DownloadState) StatusString() string {
	switch d.Status {
	case 0:
		return "Idle"
	case 1:
		return "Downloading"
	case 2:
		return "Complete"
	default:
		return "Unknown"
	}
}

// DownloadStateResponse wraps the get_download_state response.
type DownloadStateResponse struct {
	System struct {
		GetDownloadState DownloadState `json:"get_download_state"`
	} `json:"system"`
}

// DownloadFirmwareResponse wraps the download_firmware response.
type DownloadFirmwareResponse struct {
	System struct {
		DownloadFirmware struct {
			ErrCode int    `json:"err_code"`
			ErrMsg  string `json:"err_msg,omitempty"`
		} `json:"download_firmware"`
	} `json:"system"`
}

// FlashFirmwareResponse wraps the flash_downloaded_firmware response.
type FlashFirmwareResponse struct {
	System struct {
		FlashDownloadedFirmware struct {
			ErrCode int    `json:"err_code"`
			ErrMsg  string `json:"err_msg,omitempty"`
		} `json:"flash_downloaded_firmware"`
	} `json:"system"`
}
