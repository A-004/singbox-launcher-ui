package traffic

import (
	"fmt"
	"time"
)

// GeoInfo holds geo-location data fetched from an external service.
type GeoInfo struct {
	Country string `json:"country"`
	City    string `json:"city"`
}

// IsZero returns true when no geo data has been fetched yet.
func (g GeoInfo) IsZero() bool {
	return g.Country == "" && g.City == ""
}

// String returns a human-readable geo string like "Poland, Warsaw" or "—" when empty.
func (g GeoInfo) String() string {
	if g.IsZero() {
		return "—"
	}
	if g.City != "" {
		return g.Country + ", " + g.City
	}
	return g.Country
}

// TrafficStats holds a single traffic snapshot with formatted strings.
type TrafficStats struct {
	Down      float64 // bytes/sec
	Up        float64 // bytes/sec
	DownStr   string  // formatted, e.g. "45.3 MB/s"
	UpStr     string  // formatted, e.g. "2.1 MB/s"
	IsActive  bool    // true when at least one direction > 0
	Timestamp time.Time

	// ServerDelayMs is the last measured TCP ping RTT in milliseconds
	// from this client to the proxy server (pure network delay, like
	// traceroute). -1 means no measurement available yet.
	ServerDelayMs int64
	// ServerDelayStr is the formatted string for UI display,
	// e.g. "45ms" or "N/A" when no measurement available.
	ServerDelayStr string

	// ProxyTag is the display name of the currently active proxy
	// (e.g. "🇺🇸 US 01"), fetched from Clash API.
	ProxyTag string
	// ProxyAddr is the resolved server:port address being pinged,
	// e.g. "203.0.113.1:443"
	ProxyAddr string
	// PingOk is true when the last TCP ping succeeded (got SYN-ACK).
	PingOk bool

	// PingInfo is a human-readable diagnostic string showing what the
	// monitor is currently doing — e.g. "STUN OK: 2.27.42.36",
	// "TCP 443: RST 10ms", "TCP 443: timeout", "No local bind IP".
	PingInfo string

	// Geo is the geo-location (country+city) of the current egress IP.
	// Fetched periodically from an external geo-service through the tunnel.
	Geo GeoInfo
}

// NewTrafficStats creates a TrafficStats from raw bps values.
func NewTrafficStats(down, up float64, serverDelayMs int64, proxyTag, proxyAddr string, pingOk bool, pingInfo string) TrafficStats {
	delayStr := FormatServerDelay(serverDelayMs)
	return TrafficStats{
		Down:           down,
		Up:             up,
		DownStr:        FormatSpeed(down),
		UpStr:          FormatSpeed(up),
		IsActive:       down > 0 || up > 0,
		Timestamp:      time.Now(),
		ServerDelayMs:  serverDelayMs,
		ServerDelayStr: delayStr,
		ProxyTag:       proxyTag,
		ProxyAddr:      proxyAddr,
		PingOk:         pingOk,
		PingInfo:       pingInfo,
	}
}

// FormatSpeed converts bytes-per-second to a human-readable string.
// Examples: "0 B/s", "1.2 KB/s", "45.8 MB/s", "1.34 GB/s".
func FormatSpeed(bps float64) string {
	switch {
	case bps >= 1_000_000_000:
		return fmt.Sprintf("%.2f GB/s", bps/1_000_000_000)
	case bps >= 1_000_000:
		return fmt.Sprintf("%.1f MB/s", bps/1_000_000)
	case bps >= 1_000:
		return fmt.Sprintf("%.0f KB/s", bps/1_000)
	default:
		return fmt.Sprintf("%.0f B/s", bps)
	}
}

// FormatServerDelay formats a TCP ping RTT in ms to a human-readable string.
// Returns "N/A" when delayMs <= 0 (no measurement).
func FormatServerDelay(delayMs int64) string {
	if delayMs <= 0 {
		return "N/A"
	}
	return fmt.Sprintf("%dms", delayMs)
}

// PingColor returns a hex color for the ping value: green (<150ms), yellow
// (150-400ms), red (>400ms). For UI display.
func PingColor(delayMs int64) string {
	if delayMs <= 0 {
		return "#8E8E93" // gray
	}
	switch {
	case delayMs < 150:
		return "#34C759" // green
	case delayMs < 400:
		return "#FF9F0A" // yellow/orange
	default:
		return "#FF453A" // red
	}
}
