package command

// GetAvailableFirmwares returns a command to check for firmware updates.
func GetAvailableFirmwares() Command {
	return New(NSSystem, "get_available_firmwares", nil)
}

// GetDownloadState returns a command to get the firmware download status.
func GetDownloadState() Command {
	return New(NSSystem, "get_download_state", nil)
}

// DownloadFirmware returns a command to start firmware download.
func DownloadFirmware(url string) Command {
	return New(NSSystem, "download_firmware", map[string]interface{}{
		"url": url,
	})
}

// FlashFirmware returns a command to flash the downloaded firmware.
func FlashFirmware() Command {
	return New(NSSystem, "flash_downloaded_firmware", nil)
}
