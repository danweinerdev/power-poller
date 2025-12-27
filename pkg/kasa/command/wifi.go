package command

// GetScanInfo returns a command to scan for available WiFi networks.
// If refresh is true, forces a new scan rather than returning cached results.
func GetScanInfo(refresh bool) Command {
	refreshVal := 0
	if refresh {
		refreshVal = 1
	}
	return New("netif", "get_scaninfo", map[string]interface{}{
		"refresh": refreshVal,
	})
}

// SetStaInfo returns a command to connect to a WiFi network.
// keyType values:
//   - 0: No security (open network)
//   - 1: WEP
//   - 2: WPA-PSK
//   - 3: WPA2-PSK (most common)
func SetStaInfo(ssid, password string, keyType int) Command {
	return New("netif", "set_stainfo", map[string]interface{}{
		"ssid":     ssid,
		"password": password,
		"key_type": keyType,
	})
}
