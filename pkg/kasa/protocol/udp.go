package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/types"
)

// DiscoveredDevice represents a device found during network discovery.
type DiscoveredDevice struct {
	IP      string
	SysInfo *types.SysInfo
}

// Discover broadcasts a discovery request and collects responses.
// Returns all devices that respond within the timeout period.
func Discover(ctx context.Context, timeout time.Duration) ([]DiscoveredDevice, error) {
	// Create UDP connection for broadcast
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP socket: %w", err)
	}
	defer conn.Close()

	// Build the discovery command (get_sysinfo)
	cmd := map[string]interface{}{
		"system": map[string]interface{}{
			"get_sysinfo": map[string]interface{}{},
		},
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	// Encrypt the payload (UDP uses no length header, just encrypted data)
	encrypted := Encrypt(payload)

	// Get all broadcast addresses
	broadcastAddrs, err := getBroadcastAddresses()
	if err != nil {
		// Fall back to global broadcast
		broadcastAddrs = []net.IP{net.IPv4bcast}
	}

	// Send to all broadcast addresses
	for _, addr := range broadcastAddrs {
		destAddr := &net.UDPAddr{IP: addr, Port: DefaultPort}
		_, err := conn.WriteToUDP(encrypted, destAddr)
		if err != nil {
			// Log but continue - some interfaces may fail
			continue
		}
	}

	// Collect responses
	var devices []DiscoveredDevice
	var mu sync.Mutex
	seen := make(map[string]bool)

	// Create a deadline context
	deadline := time.Now().Add(timeout)
	conn.SetReadDeadline(deadline)

	// Read responses until timeout
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return devices, ctx.Err()
		default:
		}

		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			// Check if it's a timeout (expected)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			// Other errors - continue trying
			continue
		}

		if n == 0 {
			continue
		}

		// Decrypt the response
		decrypted := Decrypt(buf[:n])

		// Parse the sysinfo
		var response types.SysInfoResponse
		if err := json.Unmarshal(decrypted, &response); err != nil {
			// Invalid response - skip
			continue
		}

		ip := remoteAddr.IP.String()

		mu.Lock()
		if !seen[ip] {
			seen[ip] = true
			sysinfo := response.System.GetSysinfo
			devices = append(devices, DiscoveredDevice{
				IP:      ip,
				SysInfo: &sysinfo,
			})
		}
		mu.Unlock()
	}

	return devices, nil
}

// getBroadcastAddresses returns broadcast addresses for all network interfaces.
func getBroadcastAddresses() ([]net.IP, error) {
	var addrs []net.IP

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Skip interfaces without broadcast capability
		if iface.Flags&net.FlagBroadcast == 0 {
			continue
		}

		ifAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range ifAddrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			// Only IPv4
			ip4 := ipNet.IP.To4()
			if ip4 == nil {
				continue
			}

			// Calculate broadcast address
			broadcast := calculateBroadcast(ip4, ipNet.Mask)
			if broadcast != nil {
				addrs = append(addrs, broadcast)
			}
		}
	}

	// Always include global broadcast as fallback
	addrs = append(addrs, net.IPv4bcast)

	return addrs, nil
}

// calculateBroadcast calculates the broadcast address for a network.
func calculateBroadcast(ip net.IP, mask net.IPMask) net.IP {
	ip4 := ip.To4()
	if ip4 == nil || len(mask) != 4 {
		return nil
	}

	broadcast := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		broadcast[i] = ip4[i] | ^mask[i]
	}
	return broadcast
}
