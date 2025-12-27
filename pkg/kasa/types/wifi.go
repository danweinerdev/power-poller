package types

// ScanInfoResponse is the response from a WiFi scan command.
type ScanInfoResponse struct {
	Netif struct {
		GetScaninfo ScanInfo `json:"get_scaninfo"`
	} `json:"netif"`
}

// ScanInfo contains the list of available WiFi networks.
type ScanInfo struct {
	APList  []AccessPoint `json:"ap_list"`
	ErrCode int           `json:"err_code"`
}

// AccessPoint represents a WiFi network found during scanning.
type AccessPoint struct {
	SSID    string `json:"ssid"`
	KeyType int    `json:"key_type"` // 0=open, 1=WEP, 2=WPA, 3=WPA2
	RSSI    int    `json:"rssi"`     // Signal strength (negative dBm, e.g., -50)
}

// KeyTypeName returns a human-readable name for the key type.
func (ap AccessPoint) KeyTypeName() string {
	switch ap.KeyType {
	case 0:
		return "Open"
	case 1:
		return "WEP"
	case 2:
		return "WPA"
	case 3:
		return "WPA2"
	default:
		return "Unknown"
	}
}

// SignalQuality returns a human-readable signal quality description.
// RSSI values are typically: -30 to -50 = Excellent, -50 to -60 = Good,
// -60 to -70 = Fair, -70 to -80 = Weak, below -80 = Poor
func (ap AccessPoint) SignalQuality() string {
	switch {
	case ap.RSSI >= -50:
		return "Excellent"
	case ap.RSSI >= -60:
		return "Good"
	case ap.RSSI >= -70:
		return "Fair"
	case ap.RSSI >= -80:
		return "Weak"
	default:
		return "Poor"
	}
}

// SetStaInfoResponse is the response from a WiFi join command.
type SetStaInfoResponse struct {
	Netif struct {
		SetStainfo struct {
			ErrCode int `json:"err_code"`
		} `json:"set_stainfo"`
	} `json:"netif"`
}

// WiFi key type constants for convenience.
const (
	KeyTypeOpen = 0
	KeyTypeWEP  = 1
	KeyTypeWPA  = 2
	KeyTypeWPA2 = 3
)
